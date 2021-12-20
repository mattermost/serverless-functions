// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package function

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/alexellis/hmac/v2"
	"github.com/google/go-github/v41/github"
	handler "github.com/openfaas/templates-sdk/go-http"
	"github.com/xanzy/go-gitlab"
)

// Handle a function invocation
func Handle(req handler.Request) (handler.Response, error) {
	xHubSignature := req.Header.Get("X-Hub-Signature")
	if len(xHubSignature) == 0 {
		return sendError(http.StatusBadRequest, errors.New("missing X-Hub-Signature header"))
	}

	secretKey, err := getAPISecret("github-header-token")
	if err != nil {
		return sendError(http.StatusBadRequest, fmt.Errorf("failed to read secret"))
	}

	err = hmac.Validate(req.Body, xHubSignature, string(secretKey))
	if err != nil {
		return sendError(http.StatusBadRequest, errors.New("bad X-Hub-Signature header"))
	}

	eventType := req.Header.Get("X-GitHub-Event")

	switch eventType {
	case "ping":
		var event github.PingEvent
		if err := json.Unmarshal(req.Body, &event); err != nil {
			return sendError(http.StatusInternalServerError, fmt.Errorf("cannot parse input %v", err))
		}

		return sendStatusOk()
	case "push":
		var event github.PushEvent
		if err := json.Unmarshal(req.Body, &event); err != nil {
			return sendError(http.StatusInternalServerError, fmt.Errorf("cannot parse input %v", err))
		}

		gitlabToken, err := getAPISecret("gitlab-token")
		if err != nil {
			return sendError(http.StatusBadRequest, fmt.Errorf("failed to read secret for gitlab token"))
		}

		gitlabHost, err := getAPISecret("gitlab-host")
		if err != nil {
			return sendError(http.StatusBadRequest, fmt.Errorf("failed to read secret for gitlab host"))
		}

		git, err := gitlab.NewClient(string(gitlabToken), gitlab.WithBaseURL(string(gitlabHost)))
		if err != nil {
			return sendError(http.StatusInternalServerError, fmt.Errorf("cannot create gitlab client %v", err))
		}

		repoName := event.GetRepo().GetName()
		opt := &gitlab.ListProjectsOptions{
			// all mirrored projects are located in the namespace "mattermost/ci-only" for now if that change need to update here
			Search:           gitlab.String(fmt.Sprintf("mattermost/ci-only/%s", repoName)),
			SearchNamespaces: gitlab.Bool(true),
		}

		projects, _, err := git.Projects.ListProjects(opt)
		if err != nil {
			return sendError(http.StatusInternalServerError, fmt.Errorf("failed to list project %s: %v", repoName, err.Error()))
		}

		if len(projects) > 1 {
			return sendError(http.StatusInternalServerError, errors.New("should return just one project"))
		}

		if !projects[0].Mirror {
			return sendError(http.StatusInternalServerError, errors.New("should be a mirrored project"))
		}

		resp, err := git.Projects.StartMirroringProject(projects[0].ID)
		if err != nil {
			log.Fatal(err.Error())
			return sendError(http.StatusInternalServerError, fmt.Errorf("cannot mirror the project %v", err))
		}
		if resp.StatusCode != 200 {
			log.Fatal(err.Error())
			return sendError(http.StatusInternalServerError, fmt.Errorf("should return status 200 got %d", resp.StatusCode))
		}

		return sendStatusOk()
	default:
		return sendError(http.StatusInternalServerError, fmt.Errorf("X_Github_Event want: ['push'], got: "+eventType))
	}
}

func sendError(status int, err error) (handler.Response, error) {
	return handler.Response{
		Body:       []byte(err.Error()),
		StatusCode: status,
	}, err
}

func sendStatusOk() (handler.Response, error) {
	return handler.Response{
		Body:       []byte("ok"),
		StatusCode: http.StatusOK,
	}, nil
}

func getAPISecret(secretName string) (secretBytes []byte, err error) {
	secretBytes, err = ioutil.ReadFile("/var/openfaas/secrets/" + secretName)
	return secretBytes, err
}
