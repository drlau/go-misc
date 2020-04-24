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

	"github.com/google/go-github/v31/github"
	"golang.org/x/oauth2"
)

const EnvToken = "GITHUB_TOKEN"

var usage = `Usage: github-close-pr [options...] OWNER REPO NUMBER

Options:
  -close-comment string  Close comment
  -delete-branch bool    Delete the branch associated with the PR
`

var (
	closeCommentFlag = flag.String("close-comment", "", "")
	deleteBranchFlag = flag.Bool("delete-branch", false, "")
)

func main() {
	token := os.Getenv(EnvToken)
	if len(token) == 0 {
		log.Fatal("[ERROR] Environment variable GITHUB_TOKEN must be set")
	}

	ctx := context.Background()

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

	prNumber, err := strconv.Atoi(number)
	if err != nil || prNumber <= 0 {
		log.Fatalf("[ERROR] PR number %s must be a valid positive number\n", number)
	}

	ts := oauth2.StaticTokenSource(&oauth2.Token{
		AccessToken: token,
	})
	tc := oauth2.NewClient(oauth2.NoContext, ts)
	client := github.NewClient(tc)

	pr, _, err := client.PullRequests.Get(ctx, owner, repo, prNumber)
	if err != nil {
		log.Fatalf("[ERROR] Failed to get PR %d: %s\n", prNumber, err)
	}

	if pr.GetState() != "open" {
		log.Fatalf("[ERROR] PR %d is not open\n", prNumber)
	}

	if closeComment != "" {
		comment := &github.IssueComment{
			Body: github.String(closeComment),
		}
		_, _, err := client.Issues.CreateComment(ctx, owner, repo, prNumber, comment)
		if err != nil {
			log.Fatalf("[ERROR] Failed to comment on PR %d: %s\n", prNumber, err)
		}
	}

	pr.State = github.String("closed")
	_, _, err = client.PullRequests.Edit(ctx, owner, repo, prNumber, pr)
	if err != nil {
		log.Fatalf("[ERROR] Failed to close PR %d: %s\n", prNumber, err)
	}
	log.Printf("[INFO] Successfully closed PR %d\n", prNumber)

	if !deleteBranch {
		return
	}

	prBranchHead := pr.GetHead()
	prBranchBase := pr.GetBase()
	if prBranchHead.GetRepo().GetID() != prBranchBase.GetRepo().GetID() {
		log.Printf("[WARN] PR head repository %s and base repository %s are different - not attempting to delete PR branch\n", prBranchHead.GetRepo().GetFullName(), prBranchBase.GetRepo().GetFullName())
		return
	}
	ref := prBranchHead.GetRef()
	_, err = client.Git.DeleteRef(ctx, owner, repo, fmt.Sprintf("heads/%s", ref))
	if err != nil {
		log.Fatalf("[ERROR] Failed to delete ref %s: %s\n", ref, err)
	}
	log.Printf("[INFO] Successfully deleted ref %s\n", ref)
}
