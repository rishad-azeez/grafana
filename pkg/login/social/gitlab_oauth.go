package social

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"golang.org/x/oauth2"

	"github.com/grafana/grafana/pkg/models/roletype"
	"github.com/grafana/grafana/pkg/setting"
)

const (
	groupPerPage     = 50
	accessLevelGuest = "10"
)

type SocialGitlab struct {
	*SocialBase
	allowedGroups   []string
	apiUrl          string
	skipOrgRoleSync bool
}

type apiData struct {
	ID          int64   `json:"id"`
	Username    string  `json:"username"`
	Email       string  `json:"email"`
	State       string  `json:"state"`
	Name        string  `json:"name"`
	ConfirmedAt *string `json:"confirmed_at"` // "2020-10-02T09:39:40.882Z"
}

type userData struct {
	ID     string   `json:"sub"`
	Login  string   `json:"preferred_username"`
	Email  string   `json:"email"`
	Name   string   `json:"name"`
	Groups []string `json:"groups_direct"`

	EmailVerified  bool              `json:"email_verified"`
	Role           roletype.RoleType `json:"-"`
	IsGrafanaAdmin *bool             `json:"-"`
}

func (s *SocialGitlab) isGroupMember(groups []string) bool {
	if len(s.allowedGroups) == 0 {
		return true
	}

	for _, allowedGroup := range s.allowedGroups {
		for _, group := range groups {
			if group == allowedGroup {
				return true
			}
		}
	}

	return false
}

func (s *SocialGitlab) getGroups(ctx context.Context, client *http.Client) []string {
	groups := make([]string, 0)
	nextPage := new(int)

	for *nextPage = 1; nextPage != nil; {
		var page []string
		page, nextPage = s.getGroupsPage(ctx, client, *nextPage)
		groups = append(groups, page...)
	}

	return groups
}

// getGroupsPage returns groups and link to the next page if response is paginated
func (s *SocialGitlab) getGroupsPage(ctx context.Context, client *http.Client, nextPage int) ([]string, *int) {
	type Group struct {
		FullPath string `json:"full_path"`
	}

	groupURL, err := url.JoinPath(s.apiUrl, "/groups")
	if err != nil {
		s.log.Error("Error joining GitLab API URL", "err", err)
		return nil, nil
	}

	parsedUrl, err := url.Parse(groupURL)
	if err != nil {
		s.log.Error("Error parsing GitLab API URL", "err", err)
		return nil, nil
	}

	q := parsedUrl.Query()
	q.Set("per_page", fmt.Sprintf("%d", groupPerPage))
	q.Set("min_access_level", accessLevelGuest)
	q.Set("page", fmt.Sprintf("%d", nextPage))
	parsedUrl.RawQuery = q.Encode()

	response, err := s.httpGet(ctx, client, parsedUrl.String())
	if err != nil {
		s.log.Error("Error getting groups from GitLab API", "err", err)
		return nil, nil
	}

	respSizeString := response.Headers.Get("X-Total")
	respSize := groupPerPage
	if respSizeString != "" {
		foundSize, err := strconv.Atoi(respSizeString)
		if err != nil {
			s.log.Warn("Error parsing X-Total header from GitLab API", "err", err)
		} else {
			respSize = foundSize
		}
	}

	groups := make([]Group, 0, respSize)
	if err := json.Unmarshal(response.Body, &groups); err != nil {
		s.log.Error("Error parsing JSON from GitLab API", "err", err)
		return nil, nil
	}

	fullPaths := make([]string, len(groups))
	for i, group := range groups {
		fullPaths[i] = group.FullPath
	}

	var next *int = nil
	nextString := response.Headers.Get("X-Next-Page")
	if nextString != "" {
		foundNext, err := strconv.Atoi(nextString)
		if err != nil {
			s.log.Warn("Error parsing X-Next-Page header from GitLab API", "err", err)
		} else {
			next = &foundNext
		}
	}

	return fullPaths, next
}

func (s *SocialGitlab) UserInfo(ctx context.Context, client *http.Client, token *oauth2.Token) (*BasicUserInfo, error) {
	data, err := s.extractFromToken(ctx, client, token)
	if err != nil {
		return nil, err
	}

	// fallback to API
	if data == nil {
		var errAPI error
		data, errAPI = s.extractFromAPI(ctx, client, token)
		if errAPI != nil {
			return nil, errAPI
		}
	}

	userInfo := &BasicUserInfo{
		Id:             data.ID,
		Name:           data.Name,
		Login:          data.Login,
		Email:          data.Email,
		Groups:         data.Groups,
		Role:           data.Role,
		IsGrafanaAdmin: data.IsGrafanaAdmin,
	}

	if !s.isGroupMember(data.Groups) {
		return nil, errMissingGroupMembership
	}

	if s.allowAssignGrafanaAdmin && s.skipOrgRoleSync {
		s.log.Debug("allowAssignGrafanaAdmin and skipOrgRoleSync are both set, Grafana Admin role will not be synced, consider setting one or the other")
	}

	return userInfo, nil
}

func (s *SocialGitlab) extractFromAPI(ctx context.Context, client *http.Client, token *oauth2.Token) (*userData, error) {
	apiResp := &apiData{}
	response, err := s.httpGet(ctx, client, s.apiUrl+"/user")
	if err != nil {
		return nil, fmt.Errorf("Error getting user info: %w", err)
	}

	if err = json.Unmarshal(response.Body, &apiResp); err != nil {
		return nil, fmt.Errorf("error getting user info: %w", err)
	}

	// check confirmed_at exists and is not null
	if apiResp.ConfirmedAt == nil || *apiResp.ConfirmedAt == "" {
		return nil, fmt.Errorf("user %s's email is not confirmed", apiResp.Username)
	}

	if apiResp.State != "active" {
		return nil, fmt.Errorf("user %s is inactive", apiResp.Username)
	}

	idData := &userData{
		ID:     fmt.Sprintf("%d", apiResp.ID),
		Login:  apiResp.Username,
		Email:  apiResp.Email,
		Name:   apiResp.Name,
		Groups: s.getGroups(ctx, client),
	}

	if !s.skipOrgRoleSync {
		var grafanaAdmin bool
		role, grafanaAdmin, err := s.extractRoleAndAdmin(response.Body, idData.Groups)
		if err != nil {
			return nil, err
		}

		if s.allowAssignGrafanaAdmin {
			idData.IsGrafanaAdmin = &grafanaAdmin
		}

		idData.Role = role
	}

	if setting.Env == setting.Dev {
		s.log.Debug("Resolved ID", "data", fmt.Sprintf("%+v", idData))
	}

	return idData, nil
}

func (s *SocialGitlab) extractFromToken(ctx context.Context, client *http.Client, token *oauth2.Token) (*userData, error) {
	s.log.Debug("Extracting user info from OAuth token")

	idToken := token.Extra("id_token")
	if idToken == nil {
		s.log.Debug("No id_token found, defaulting to API access", "token", token)
		return nil, nil
	}

	rawJSON, err := s.retrieveRawIDToken(idToken)
	if err != nil {
		s.log.Warn("Error retrieving id_token", "error", err, "token", fmt.Sprintf("%+v", idToken))
		return nil, nil
	}

	s.log.Debug("Received id_token", "raw_json", string(rawJSON))
	var data userData
	if err := json.Unmarshal(rawJSON, &data); err != nil {
		s.log.Warn("Error decoding id_token JSON", "raw_json", string(rawJSON), "error", err)
		return nil, nil
	}

	// check email_verified
	if !data.EmailVerified {
		return nil, fmt.Errorf("user %s's email is not confirmed", data.Login)
	}

	userInfo, err := s.retrieveUserInfo(ctx, client)
	if err != nil {
		s.log.Warn("Error retrieving groups from userinfo. Using only token provided groups", "error", err)
	} else {
		s.log.Debug("Retrieved groups from userinfo", "sub", userInfo.Sub,
			"original_groups", data.Groups, "groups", userInfo.Groups)
		data.Groups = userInfo.Groups
	}

	if !s.skipOrgRoleSync {
		role, grafanaAdmin, errRole := s.extractRoleAndAdmin(rawJSON, data.Groups)
		if errRole != nil {
			return nil, errRole
		}

		if s.allowAssignGrafanaAdmin {
			data.IsGrafanaAdmin = &grafanaAdmin
		}

		data.Role = role
	}

	s.log.Debug("Resolved user data", "data", fmt.Sprintf("%+v", data))
	return &data, nil
}

type userInfoResponse struct {
	Sub               string   `json:"sub"`
	SubLegacy         string   `json:"sub_legacy"`
	Name              string   `json:"name"`
	Nickname          string   `json:"nickname"`
	PreferredUsername string   `json:"preferred_username"`
	Email             string   `json:"email"`
	EmailVerified     bool     `json:"email_verified"`
	Profile           string   `json:"profile"`
	Picture           string   `json:"picture"`
	Groups            []string `json:"groups"`
	OwnerGroups       []string `json:"https://gitlab.org/claims/groups/owner"`
}

// retrieve and parse /oauth/userinfo
func (s *SocialGitlab) retrieveUserInfo(ctx context.Context, client *http.Client) (*userInfoResponse, error) {
	userInfoURL := strings.TrimSuffix(s.Endpoint.AuthURL, "/oauth/authorize") + "/oauth/userinfo"

	resp, err := s.httpGet(ctx, client, userInfoURL)
	if err != nil {
		return nil, err
	}

	var userInfo userInfoResponse
	if err := json.Unmarshal(resp.Body, &userInfo); err != nil {
		return nil, err
	}

	return &userInfo, nil
}