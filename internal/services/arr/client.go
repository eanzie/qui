// Copyright (c) 2025-2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package arr

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/autobrr/qui/internal/models"
	"github.com/autobrr/qui/pkg/httphelpers"
)

const (
	defaultTimeout   = 15 * time.Second
	defaultUserAgent = "qui/1.0"
)

// Client is an HTTP client for communicating with Sonarr/Radarr v3 API
type Client struct {
	instanceType models.ArrInstanceType
	baseURL      string
	apiKey       string
	basicUser    string
	basicPass    string
	httpClient   *http.Client
	timeout      time.Duration
}

// NewClient creates a new ARR API client
func NewClient(baseURL, apiKey string, basicUsername, basicPassword *string, instanceType models.ArrInstanceType, timeoutSeconds int) *Client {
	timeout := defaultTimeout
	if timeoutSeconds > 0 {
		timeout = time.Duration(timeoutSeconds) * time.Second
	}

	return &Client{
		instanceType: instanceType,
		baseURL:      strings.TrimRight(baseURL, "/"),
		apiKey:       apiKey,
		basicUser:    strings.TrimSpace(stringOrEmpty(basicUsername)),
		basicPass:    strings.TrimSpace(stringOrEmpty(basicPassword)),
		httpClient: &http.Client{
			Timeout: timeout,
		},
		timeout: timeout,
	}
}

// Ping tests connectivity to the ARR instance via GET /api/v3/system/status
func (c *Client) Ping(ctx context.Context) error {
	endpoint := c.baseURL + "/api/v3/system/status"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(req)

	resp, err := c.httpClient.Do(req) //nolint:bodyclose // closed by DrainAndClose
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer httphelpers.DrainAndClose(resp)

	if resp.StatusCode == http.StatusUnauthorized {
		return errors.New("authentication failed: invalid API key")
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var status SystemStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	// Validate we got a valid response with app name
	if status.AppName == "" {
		return fmt.Errorf("invalid response: missing appName")
	}

	return nil
}

// ParseTitle calls the parse endpoint to resolve a title to external IDs
// For Sonarr: GET /api/v3/parse?title=<title>
// For Radarr: GET /api/v3/parse?title=<title>
func (c *Client) ParseTitle(ctx context.Context, title string) (*models.ExternalIDs, error) {
	result, err := c.ParseTitleLookupResult(ctx, title)
	if result == nil {
		return nil, err
	}
	return result.IDs, err
}

// ParseTitleLookupResult calls the parse endpoint to resolve a title to IDs and ARR title aliases.
func (c *Client) ParseTitleLookupResult(ctx context.Context, title string) (*ExternalIDsLookupResult, error) {
	endpoint := c.baseURL + "/api/v3/parse"

	// Build URL with query parameter
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to parse endpoint URL: %w", err)
	}
	q := u.Query()
	q.Set("title", title)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(req)

	resp, err := c.httpClient.Do(req) //nolint:bodyclose // closed by DrainAndClose
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer httphelpers.DrainAndClose(resp)

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, errors.New("authentication failed: invalid API key")
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	// Parse based on instance type
	switch c.instanceType {
	case models.ArrInstanceTypeSonarr:
		return c.parseSonarrResponse(ctx, resp.Body)
	case models.ArrInstanceTypeRadarr:
		return c.parseRadarrResponse(ctx, resp.Body)
	default:
		return nil, fmt.Errorf("unsupported instance type: %s", c.instanceType)
	}
}

// LookupByTerm queries the ARR title-search endpoint (Radarr /api/v3/movie/lookup,
// Sonarr /api/v3/series/lookup) and returns IDs/titles from the best-matching
// candidate, or nil when no candidate matches term (and year, when known).
func (c *Client) LookupByTerm(ctx context.Context, term string, year int) (*ExternalIDsLookupResult, error) {
	params := url.Values{}
	params.Set("term", term)
	switch c.instanceType {
	case models.ArrInstanceTypeRadarr:
		var movies []RadarrMovie
		if err := c.getJSON(ctx, "/api/v3/movie/lookup", params, &movies); err != nil {
			return nil, err
		}
		return lookupResultFromRadarrMovie(selectRadarrLookupMatch(term, year, movies)), nil
	case models.ArrInstanceTypeSonarr:
		var series []SonarrSeries
		if err := c.getJSON(ctx, "/api/v3/series/lookup", params, &series); err != nil {
			return nil, err
		}
		return c.sonarrSeriesLookupResult(ctx, selectSonarrLookupMatch(term, year, series)), nil
	default:
		return nil, fmt.Errorf("unsupported instance type: %s", c.instanceType)
	}
}

func (c *Client) sonarrSeriesLookupResult(ctx context.Context, series *SonarrSeries) *ExternalIDsLookupResult {
	result := lookupResultFromSonarrSeries(series)
	if series == nil || series.ID <= 0 {
		return result
	}

	fullSeries, err := c.sonarrSeriesByID(ctx, series.ID)
	if err != nil {
		return result
	}

	return mergeLookupResults(result, lookupResultFromSonarrSeries(fullSeries))
}

func (c *Client) sonarrSeriesByID(ctx context.Context, id int) (*SonarrSeries, error) {
	var series SonarrSeries
	if err := c.getJSON(ctx, fmt.Sprintf("/api/v3/series/%d", id), nil, &series); err != nil {
		return nil, err
	}
	return &series, nil
}

func (c *Client) radarrMovieLookupResult(ctx context.Context, parseResp *RadarrParseResponse) *ExternalIDsLookupResult {
	if parseResp == nil {
		return nil
	}

	result := parseResp.ExtractLookupResult()
	if parseResp.Movie == nil || parseResp.Movie.ID <= 0 {
		return result
	}

	fullMovie, err := c.radarrMovieByID(ctx, parseResp.Movie.ID)
	if err != nil {
		return result
	}

	return mergeLookupResults(result, lookupResultFromRadarrMovie(fullMovie))
}

func (c *Client) radarrMovieByID(ctx context.Context, id int) (*RadarrMovie, error) {
	var movie RadarrMovie
	if err := c.getJSON(ctx, fmt.Sprintf("/api/v3/movie/%d", id), nil, &movie); err != nil {
		return nil, err
	}
	return &movie, nil
}

func mergeLookupResults(base, hydrated *ExternalIDsLookupResult) *ExternalIDsLookupResult {
	if base == nil {
		return hydrated
	}
	if hydrated == nil {
		return base
	}

	result := &ExternalIDsLookupResult{
		IDs:    mergeExternalIDs(base.IDs, hydrated.IDs),
		Titles: append([]string(nil), base.Titles...),
	}
	for _, title := range hydrated.Titles {
		addUniqueTitle(&result.Titles, title)
	}
	if result.IDs == nil && len(result.Titles) == 0 {
		return nil
	}
	return result
}

func mergeExternalIDs(base, hydrated *models.ExternalIDs) *models.ExternalIDs {
	if base == nil {
		return hydrated
	}
	if hydrated == nil {
		return base
	}

	ids := *base
	if hydrated.TVDbID > 0 {
		ids.TVDbID = hydrated.TVDbID
	}
	if hydrated.TVMazeID > 0 {
		ids.TVMazeID = hydrated.TVMazeID
	}
	if hydrated.TMDbID > 0 {
		ids.TMDbID = hydrated.TMDbID
	}
	if hydrated.IMDbID != "" && hydrated.IMDbID != "0" {
		ids.IMDbID = hydrated.IMDbID
	}
	if ids.IsEmpty() {
		return nil
	}
	return &ids
}

// ParseSonarrTitle returns the full Sonarr parse response for TV lookups that need the series ID.
func (c *Client) ParseSonarrTitle(ctx context.Context, title string) (*SonarrParseResponse, error) {
	if c.instanceType != models.ArrInstanceTypeSonarr {
		return nil, fmt.Errorf("unsupported instance type for Sonarr parse: %s", c.instanceType)
	}

	var parseResp SonarrParseResponse
	params := url.Values{}
	params.Set("title", title)
	if err := c.getJSON(ctx, "/api/v3/parse", params, &parseResp); err != nil {
		if strings.Contains(err.Error(), "failed to decode response") {
			return nil, fmt.Errorf("failed to decode Sonarr parse response: %w", err)
		}
		return nil, err
	}

	return &parseResp, nil
}

// GetSonarrSeasonEpisodes fetches episodes for a specific Sonarr series season.
func (c *Client) GetSonarrSeasonEpisodes(ctx context.Context, seriesID, seasonNumber int) ([]SonarrEpisodeResource, error) {
	if c.instanceType != models.ArrInstanceTypeSonarr {
		return nil, fmt.Errorf("unsupported instance type for Sonarr episodes: %s", c.instanceType)
	}

	var episodes []SonarrEpisodeResource
	params := url.Values{}
	params.Set("seriesId", strconv.Itoa(seriesID))
	params.Set("seasonNumber", strconv.Itoa(seasonNumber))
	if err := c.getJSON(ctx, "/api/v3/episode", params, &episodes); err != nil {
		if strings.Contains(err.Error(), "failed to decode response") {
			return nil, fmt.Errorf("failed to decode Sonarr episode response: %w", err)
		}
		return nil, err
	}

	return episodes, nil
}

func (c *Client) getJSON(ctx context.Context, path string, params url.Values, target any) error {
	u, err := url.Parse(c.baseURL + path)
	if err != nil {
		return fmt.Errorf("failed to parse endpoint URL: %w", err)
	}
	u.RawQuery = params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req) //nolint:bodyclose // closed by DrainAndClose
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer httphelpers.DrainAndClose(resp)

	if resp.StatusCode == http.StatusUnauthorized {
		return errors.New("authentication failed: invalid API key")
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}
	return nil
}

// parseSonarrResponse parses a Sonarr parse response and extracts external IDs
func (c *Client) parseSonarrResponse(ctx context.Context, body io.Reader) (*ExternalIDsLookupResult, error) {
	var parseResp SonarrParseResponse
	if err := json.NewDecoder(body).Decode(&parseResp); err != nil {
		return nil, fmt.Errorf("failed to decode Sonarr parse response: %w", err)
	}

	return c.sonarrSeriesLookupResult(ctx, parseResp.Series), nil
}

// parseRadarrResponse parses a Radarr parse response and extracts external IDs
func (c *Client) parseRadarrResponse(ctx context.Context, body io.Reader) (*ExternalIDsLookupResult, error) {
	var parseResp RadarrParseResponse
	if err := json.NewDecoder(body).Decode(&parseResp); err != nil {
		return nil, fmt.Errorf("failed to decode Radarr parse response: %w", err)
	}

	return c.radarrMovieLookupResult(ctx, &parseResp), nil
}

// setHeaders sets the required headers for ARR API requests
func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("X-Api-Key", c.apiKey)
	req.Header.Set("User-Agent", defaultUserAgent)
	req.Header.Set("Accept", "application/json")
	if c.basicUser != "" {
		req.SetBasicAuth(c.basicUser, c.basicPass)
	}
}

// InstanceType returns the ARR instance type this client is configured for
func (c *Client) InstanceType() models.ArrInstanceType {
	return c.instanceType
}

// BaseURL returns the base URL this client is configured for
func (c *Client) BaseURL() string {
	return c.baseURL
}

// GetSeries fetches all series from Sonarr via GET /api/v3/series.
func (c *Client) GetSeries(ctx context.Context) ([]SonarrSeriesResponse, error) {
	var result []SonarrSeriesResponse
	if err := c.getJSON(ctx, "/api/v3/series", nil, &result); err != nil {
		return nil, fmt.Errorf("get series: %w", err)
	}
	return result, nil
}

// GetEpisodeFiles fetches episode files for a series via GET /api/v3/episodefile?seriesId={id}.
func (c *Client) GetEpisodeFiles(ctx context.Context, seriesID int) ([]SonarrEpisodeFileResponse, error) {
	var result []SonarrEpisodeFileResponse
	endpoint := fmt.Sprintf("/api/v3/episodefile?seriesId=%d", seriesID)
	if err := c.getJSON(ctx, endpoint, nil, &result); err != nil {
		return nil, fmt.Errorf("get episode files: %w", err)
	}
	return result, nil
}

// GetEpisodes fetches episodes for a series via GET /api/v3/episode?seriesId={id}.
func (c *Client) GetEpisodes(ctx context.Context, seriesID int) ([]SonarrEpisodeResponse, error) {
	var result []SonarrEpisodeResponse
	endpoint := fmt.Sprintf("/api/v3/episode?seriesId=%d", seriesID)
	if err := c.getJSON(ctx, endpoint, nil, &result); err != nil {
		return nil, fmt.Errorf("get episodes: %w", err)
	}
	return result, nil
}

// SetEpisodesMonitored bulk-updates the monitored state of episodes via PUT /api/v3/episode/monitor.
func (c *Client) SetEpisodesMonitored(ctx context.Context, episodeIDs []int, monitored bool) error {
	body := map[string]any{
		"episodeIds": episodeIDs,
		"monitored":  monitored,
	}
	return c.putJSON(ctx, "/api/v3/episode/monitor", body)
}

// GetSeriesHistory fetches grabbed history for a series via GET /api/v3/history/series?seriesId={id}&eventType=grabbed.
func (c *Client) GetSeriesHistory(ctx context.Context, seriesID int) ([]SonarrHistoryRecord, error) {
	var result []SonarrHistoryRecord
	endpoint := fmt.Sprintf("/api/v3/history/series?seriesId=%d&eventType=grabbed", seriesID)
	if err := c.getJSON(ctx, endpoint, nil, &result); err != nil {
		return nil, fmt.Errorf("get series history: %w", err)
	}
	return result, nil
}

// GetMovies fetches all movies from Radarr via GET /api/v3/movie.
func (c *Client) GetMovies(ctx context.Context) ([]RadarrMovieResponse, error) {
	var result []RadarrMovieResponse
	if err := c.getJSON(ctx, "/api/v3/movie", nil, &result); err != nil {
		return nil, fmt.Errorf("get movies: %w", err)
	}
	return result, nil
}

// GetMovieFiles fetches movie files for the given movie IDs via GET /api/v3/moviefile?movieId={ids}.
// Unlike the inline movieFile in the /movie endpoint, this endpoint populates CustomFormatScore.
func (c *Client) GetMovieFiles(ctx context.Context, movieIDs []int) ([]RadarrMovieFileResponse, error) {
	if len(movieIDs) == 0 {
		return nil, nil
	}
	params := make([]string, len(movieIDs))
	for i, id := range movieIDs {
		params[i] = fmt.Sprintf("movieId=%d", id)
	}
	endpoint := "/api/v3/moviefile?" + strings.Join(params, "&")
	var result []RadarrMovieFileResponse
	if err := c.getJSON(ctx, endpoint, nil, &result); err != nil {
		return nil, fmt.Errorf("get movie files: %w", err)
	}
	return result, nil
}

// GetMovieHistory fetches grabbed history for a movie via GET /api/v3/history/movie?movieId={id}&eventType=grabbed.
func (c *Client) GetMovieHistory(ctx context.Context, movieID int) ([]RadarrHistoryRecord, error) {
	var result []RadarrHistoryRecord
	endpoint := fmt.Sprintf("/api/v3/history/movie?movieId=%d&eventType=grabbed", movieID)
	if err := c.getJSON(ctx, endpoint, nil, &result); err != nil {
		return nil, fmt.Errorf("get movie history: %w", err)
	}
	return result, nil
}

// postJSON performs a POST request with a JSON body and decodes the JSON response.
func (c *Client) postJSON(ctx context.Context, path string, body any, target any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %w", err)
	}

	endpoint := c.baseURL + path

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req) //nolint:bodyclose // closed by DrainAndClose
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer httphelpers.DrainAndClose(resp)

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("authentication failed: invalid API key")
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}
	return nil
}

// putJSON performs a PUT request with a JSON body (no response decoding).
func (c *Client) putJSON(ctx context.Context, path string, body any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %w", err)
	}

	endpoint := c.baseURL + path

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req) //nolint:bodyclose // closed by DrainAndClose
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer httphelpers.DrainAndClose(resp)

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("authentication failed: invalid API key")
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// GetQualityProfiles fetches quality profiles via GET /api/v3/qualityprofile.
func (c *Client) GetQualityProfiles(ctx context.Context) ([]QualityProfileResponse, error) {
	var result []QualityProfileResponse
	if err := c.getJSON(ctx, "/api/v3/qualityprofile", nil, &result); err != nil {
		return nil, fmt.Errorf("get quality profiles: %w", err)
	}
	return result, nil
}

// GetTags fetches all tags via GET /api/v3/tag.
func (c *Client) GetTags(ctx context.Context) ([]TagResponse, error) {
	var result []TagResponse
	if err := c.getJSON(ctx, "/api/v3/tag", nil, &result); err != nil {
		return nil, fmt.Errorf("get tags: %w", err)
	}
	return result, nil
}

// CreateTag creates a tag via POST /api/v3/tag.
func (c *Client) CreateTag(ctx context.Context, label string) (*TagResponse, error) {
	var result TagResponse
	body := map[string]string{"label": label}
	if err := c.postJSON(ctx, "/api/v3/tag", body, &result); err != nil {
		return nil, fmt.Errorf("create tag: %w", err)
	}
	return &result, nil
}

// GetSeriesByID fetches a full series object as raw JSON via GET /api/v3/series/{id}.
func (c *Client) GetSeriesByID(ctx context.Context, id int) (json.RawMessage, error) {
	var result json.RawMessage
	endpoint := fmt.Sprintf("/api/v3/series/%d", id)
	if err := c.getJSON(ctx, endpoint, nil, &result); err != nil {
		return nil, fmt.Errorf("get series by id: %w", err)
	}
	return result, nil
}

// GetMovieByID fetches a full movie object as raw JSON via GET /api/v3/movie/{id}.
func (c *Client) GetMovieByID(ctx context.Context, id int) (json.RawMessage, error) {
	var result json.RawMessage
	endpoint := fmt.Sprintf("/api/v3/movie/%d", id)
	if err := c.getJSON(ctx, endpoint, nil, &result); err != nil {
		return nil, fmt.Errorf("get movie by id: %w", err)
	}
	return result, nil
}

// UpdateSeriesFields fetches a series, merges the given field updates, and PUTs it back.
// This preserves all existing fields while only changing the specified ones.
func (c *Client) UpdateSeriesFields(ctx context.Context, seriesID int, updates map[string]any) error {
	raw, err := c.GetSeriesByID(ctx, seriesID)
	if err != nil {
		return fmt.Errorf("fetch series for update: %w", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return fmt.Errorf("unmarshal series JSON: %w", err)
	}

	maps.Copy(obj, updates)

	endpoint := fmt.Sprintf("/api/v3/series/%d", seriesID)
	if err := c.putJSON(ctx, endpoint, obj); err != nil {
		return fmt.Errorf("update series: %w", err)
	}
	return nil
}

// UpdateMovieFields fetches a movie, merges the given field updates, and PUTs it back.
// This preserves all existing fields while only changing the specified ones.
func (c *Client) UpdateMovieFields(ctx context.Context, movieID int, updates map[string]any) error {
	raw, err := c.GetMovieByID(ctx, movieID)
	if err != nil {
		return fmt.Errorf("fetch movie for update: %w", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return fmt.Errorf("unmarshal movie JSON: %w", err)
	}

	maps.Copy(obj, updates)

	endpoint := fmt.Sprintf("/api/v3/movie/%d", movieID)
	if err := c.putJSON(ctx, endpoint, obj); err != nil {
		return fmt.Errorf("update movie: %w", err)
	}
	return nil
}

// SendCommand sends a command to the ARR instance via POST /api/v3/command.
func (c *Client) SendCommand(ctx context.Context, name string, params map[string]any) (*CommandResponse, error) {
	body := map[string]any{"name": name}
	maps.Copy(body, params)
	var result CommandResponse
	if err := c.postJSON(ctx, "/api/v3/command", body, &result); err != nil {
		return nil, fmt.Errorf("send command %s: %w", name, err)
	}
	return &result, nil
}

// GetCommandStatus checks the status of a command via GET /api/v3/command/{id}.
func (c *Client) GetCommandStatus(ctx context.Context, id int) (*CommandResponse, error) {
	var result CommandResponse
	endpoint := fmt.Sprintf("/api/v3/command/%d", id)
	if err := c.getJSON(ctx, endpoint, nil, &result); err != nil {
		return nil, fmt.Errorf("get command status: %w", err)
	}
	return &result, nil
}

func stringOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
