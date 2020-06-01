/*
Command 'github-list-pr' lists all PRs matching a Github search query.

  $ github-list-pr QUERY

The resulting PR list is dumped as a JSON array. Each object contains the PR number, title, state and URL.

To use this command, a GitHub API token must be set to the env var GITHUB_TOKEN.

To install, use go get

  $ go get github.com/drlau/go-misc/github-list-pr

*/
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/shurcooL/githubv4"
	"golang.org/x/oauth2"
)

const (
	EnvToken = "GITHUB_TOKEN"
	usage    = `Usage: github-list-pr QUERY`
)

type PullRequest struct {
	Number int    `json:number`
	Title  string `json:title`
	State  string `json:state`
	URL    string `json:url`
}

func main() {
	token := os.Getenv(EnvToken)
	if len(token) == 0 {
		log.Fatal("[ERROR] Environment variable GITHUB_TOKEN must be set")
	}

	flag.Usage = func() {
		fmt.Fprint(os.Stderr, usage)
	}
	flag.Parse()

	args := flag.Args()
	if len(args) != 1 {
		log.Println("[ERROR] Invalid argument")
		fmt.Fprint(os.Stderr, usage)
		return
	}
	query := args[0]

	ts := oauth2.StaticTokenSource(&oauth2.Token{
		AccessToken: token,
	})
	tc := oauth2.NewClient(oauth2.NoContext, ts)
	v4 := githubv4.NewClient(tc)

	var q struct {
		Search struct {
			IssueCount int
			Nodes      []struct {
				PullRequest `graphql:"... on PullRequest"`
			}
			PageInfo struct {
				EndCursor   githubv4.String
				HasNextPage bool
			}
		} `graphql:"search(query: $query, type: $type, first: 100, after: $resultCursor)"`
	}

	variables := map[string]interface{}{
		"query":        githubv4.String(query),
		"type":         githubv4.SearchTypeIssue,
		"resultCursor": (*githubv4.String)(nil), // Null after argument to get first page.
	}

	var result []PullRequest
	for {
		err := v4.Query(context.Background(), &q, variables)
		if err != nil {
			log.Fatalf("[ERROR] Error querying Github API: %s", err)
		}
		for _, n := range q.Search.Nodes {
			result = append(result, n.PullRequest)
		}

		if !q.Search.PageInfo.HasNextPage {
			break
		}
		variables["resultCursor"] = githubv4.NewString(q.Search.PageInfo.EndCursor)
	}

	res, err := json.Marshal(result)
	if err != nil {
		log.Fatalf("[ERROR] Error marshalling result into JSON: %s", err)
	}
	fmt.Printf(string(res))
}
