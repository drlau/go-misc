/*
Command 'github-close-pr' closes a PR on GitHub.

  $ github-close-pr [OPTIONS...] ORG REPO BRANCH

You can add a comment to close the PR with and delete the remote branch associated 
with the branch if it is the same repo as the base branch.

To use this command, a GitHub API token must be set to the env var GITHUB_TOKEN.

To install, use go get

  $ go get github.com/drlau/go-misc/github-close-pr

*/
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/shurcooL/githubv4"
	"golang.org/x/oauth2"
)

const EnvToken = "GITHUB_TOKEN"

var usage = `Usage: github-close-pr [options...] OWNER REPO NUMBER

Options:
  -close-comment string  Close comment
  -delete-branch bool    Delete the branch associated with the PR
  -strict        bool    Fail if the PR is not open
`

var (
	closeCommentFlag = flag.String("close-comment", "", "")
	deleteBranchFlag = flag.Bool("delete-branch", false, "")
	strictFlag = flag.Bool("strict", false, "")
)

type PullRequest struct {
	ID          string
	Number      int
	Title       string
	State       string

	BaseRepository struct {
		ID string
		Name string
	}
	HeadRef struct {
		ID string
		Name string
	}
	HeadRepository struct {
		ID string
		Name string
	}
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
	if len(args) != 3 {
		log.Println("[ERROR] Invalid argument")
		fmt.Fprint(os.Stderr, usage)
		return
	}
	owner, repo, number := args[0], args[1], args[2]

	closeComment := *closeCommentFlag
	deleteBranch := *deleteBranchFlag
	strict := *strictFlag

	prNumber, err := strconv.Atoi(number)
	if err != nil || prNumber <= 0 {
		log.Fatalf("[ERROR] PR number %s must be a valid positive number\n", number)
	}

	ts := oauth2.StaticTokenSource(&oauth2.Token{
		AccessToken: token,
	})
	tc := oauth2.NewClient(oauth2.NoContext, ts)
	v4 := githubv4.NewClient(tc)

	pr, err := getPRIDByNumber(v4, owner, repo, prNumber)
	if err != nil {
		log.Fatalf("[ERROR] Failed to get PR ID for PR %d: %s\n", prNumber, err)
	}

	if pr.State != "OPEN" {
		if strict {
			log.Fatalf("[ERROR] PR %d is not open - not attempting any action\n", prNumber)
		}
		log.Printf("[WARN] PR %d is not open - not attempting any action\n", prNumber)
		return
	}

	if closeComment != "" {
		err = commentOnPR(v4, pr.ID, closeComment)
		if err != nil {
			log.Fatalf("[ERROR] Failed to comment on PR %d: %s\n", prNumber, err)
		}
	}

	err = closePR(v4, pr.ID)
	if err != nil {
		log.Fatalf("[ERROR] Failed to close PR %d: %s\n", prNumber, err)
	}
	log.Printf("[INFO] Successfully closed PR %d\n", prNumber)

	if !deleteBranch {
		return
	}

	if pr.HeadRepository.ID != pr.BaseRepository.ID {
		log.Printf("[WARN] PR head repository %s and base repository %s are different - not attempting to delete PR branch\n", pr.HeadRepository.Name, pr.BaseRepository.Name)
		return
	}
	err = deleteRef(v4, pr.HeadRef.ID)
	if err != nil {
		log.Fatalf("[ERROR] Failed to delete ref %s: %s\n", pr.HeadRef.Name, err)
	}
	log.Printf("[INFO] Successfully deleted ref %s\n", pr.HeadRef.Name)
}

func getPRIDByNumber(v4 *githubv4.Client, owner, repo string, number int) (*PullRequest, error) {
	var response struct {
		Repository struct {
			PullRequest PullRequest `graphql:"pullRequest(number: $pr_number)"`
		} `graphql:"repository(owner: $owner, name: $repo)"`
	}

	variables := map[string]interface{}{
		"owner":     githubv4.String(owner),
		"repo":      githubv4.String(repo),
		"pr_number": githubv4.Int(number),
	}

	err := v4.Query(context.Background(), &response, variables)
	if err != nil {
		return nil, err
	}

	return &response.Repository.PullRequest, nil
}

func closePR(v4 *githubv4.Client, id string) error {
	var mutation struct {
		ClosePullRequest struct {
			PullRequest struct {
				ID githubv4.ID
			}
		} `graphql:"closePullRequest(input: $input)"`
	}

	input := githubv4.ClosePullRequestInput{
		PullRequestID: id,
	}

	err := v4.Mutate(context.Background(), &mutation, input, nil)

	return err
}

func commentOnPR(v4 *githubv4.Client, id, body string) error {
	var mutation struct {
		AddComment struct {
			Subject struct {
				ID githubv4.ID
			}
		} `graphql:"addComment(input: $input)"`
	}

	input := githubv4.AddCommentInput{
		SubjectID: id,
		Body: githubv4.String(body),
	}

	err := v4.Mutate(context.Background(), &mutation, input, nil)

	return err
}

func deleteRef(v4 *githubv4.Client, id string) error {
	var mutation struct {
		DeleteRef struct{
			ClientMutationID string
		} `graphql:"deleteRef(input: $input)"`
	}

	input := githubv4.DeleteRefInput{
		RefID: id,
	}

	err := v4.Mutate(context.Background(), &mutation, input, nil)

	return err
}