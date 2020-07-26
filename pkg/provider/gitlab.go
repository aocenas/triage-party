package provider

import (
	"encoding/json"
	"fmt"
	"github.com/google/triage-party/pkg/constants"
	"github.com/google/triage-party/pkg/models"
	"github.com/xanzy/go-gitlab"
	"log"
	"strconv"
	"time"
)

type GitlabProvider struct {
	client *gitlab.Client
}

func initGitlab(c Config) {
	cl, err := gitlab.NewClient(mustReadToken(*c.GitlabTokenFile, constants.GitlabTokenEnvVar))
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	gitlabProvider = &GitlabProvider{
		client: cl,
	}
}

func (p *GitlabProvider) getListProjectIssuesOptions(sp models.SearchParams) *gitlab.ListProjectIssuesOptions {
	var state *string
	if sp.IssueListByRepoOptions.State == constants.OpenState {
		s := constants.OpenedState
		state = &s
	}
	return &gitlab.ListProjectIssuesOptions{
		ListOptions:  p.getListOptions(sp.IssueListByRepoOptions.ListOptions),
		State:        state,
		CreatedAfter: &sp.IssueListByRepoOptions.Since,
	}
}

func (p *GitlabProvider) getListOptions(m models.ListOptions) gitlab.ListOptions {
	return gitlab.ListOptions{
		Page:    m.Page,
		PerPage: m.PerPage,
	}
}

func (p *GitlabProvider) getIssues(i []*gitlab.Issue) []*models.Issue {
	r := make([]*models.Issue, len(i))
	for k, v := range i {
		m := models.Issue{}
		b, err := json.Marshal(v)
		if err != nil {
			fmt.Println(err)
		}
		err = json.Unmarshal(b, &m)
		if err != nil {
			fmt.Println(err)
		}
		r[k] = &m
	}
	return r
}

func (p *GitlabProvider) getRate(i *gitlab.Response) models.Rate {
	l, err := strconv.Atoi(i.Header.Get(constants.GitlabRateLimitHeader))
	if err != nil {
		fmt.Println(err)
	}
	r, err := strconv.Atoi(i.Header.Get(constants.GitlabRateLimitRemainingHeader))
	if err != nil {
		fmt.Println(err)
	}
	rs, err := strconv.Atoi(i.Header.Get(constants.GitlabRateLimitResetHeader))
	if err != nil {
		fmt.Println(err)
	}
	tm := time.Unix(int64(rs), 0)
	return models.Rate{
		Limit:     l,
		Remaining: r,
		Reset:     models.Timestamp{tm},
	}
}

func (p *GitlabProvider) getResponse(i *gitlab.Response) *models.Response {
	r := models.Response{
		NextPage: i.NextPage,
		Rate:     p.getRate(i),
	}
	return &r
}

// https://docs.gitlab.com/ee/api/issues.html#list-project-issues
func (p *GitlabProvider) IssuesListByRepo(sp models.SearchParams) (i []*models.Issue, r *models.Response, err error) {
	opt := p.getListProjectIssuesOptions(sp)
	is, gr, err := p.client.Issues.ListProjectIssues(p.getProjectId(sp.Repo), opt)
	i = p.getIssues(is)
	r = p.getResponse(gr)
	return
}

func (p *GitlabProvider) getListIssueNotesOptions(sp models.SearchParams) *gitlab.ListIssueNotesOptions {
	return &gitlab.ListIssueNotesOptions{
		ListOptions: p.getListOptions(sp.IssueListCommentsOptions.ListOptions),
	}
}

func (p *GitlabProvider) getUserFromNote(i *gitlab.Note) *models.User {
	id := int64(i.ID)
	return &models.User{
		ID:        &id,
		Name:      &i.Author.Name,
		Login:     &i.Author.Username, // TODO need to clarify
		Email:     &i.Author.Email,
		AvatarURL: &i.Author.AvatarURL,
		HTMLURL:   &i.Author.WebURL, // TODO need to clarify
	}
}

func (p *GitlabProvider) getIssueComments(i []*gitlab.Note) []*models.IssueComment {
	r := make([]*models.IssueComment, len(i))
	for k, v := range i {
		m := &models.IssueComment{
			User:      p.getUserFromNote(v),
			Body:      &v.Body,
			CreatedAt: v.CreatedAt,
			UpdatedAt: v.UpdatedAt,
		}
		r[k] = m
	}
	return r
}

// https://docs.gitlab.com/ce/api/notes.html#list-project-issue-notes
func (p *GitlabProvider) IssuesListComments(sp models.SearchParams) (i []*models.IssueComment, r *models.Response, err error) {
	opt := p.getListIssueNotesOptions(sp)
	in, gr, err := p.client.Notes.ListIssueNotes(p.getProjectId(sp.Repo), sp.IssueNumber, opt)
	i = p.getIssueComments(in)
	r = p.getResponse(gr)
	return
}

func (p *GitlabProvider) IssuesListIssueTimeline(sp models.SearchParams) (i []*models.Timeline, r *models.Response, err error) {
	// TODO need discuss - gitlab dont provide events by issue number (Issues, Merge Requests)
	return
}

func (p *GitlabProvider) getListProjectMergeRequestsOptions(sp models.SearchParams) *gitlab.ListProjectMergeRequestsOptions {
	var orderBy string
	if sp.PullRequestListOptions.Sort == constants.UpdatedSortOption {
		orderBy = constants.UpdatedAtSortOption
	} else {
		orderBy = constants.CreatedAtSortOption
	}
	return &gitlab.ListProjectMergeRequestsOptions{
		ListOptions: p.getListOptions(sp.PullRequestListOptions.ListOptions),
		Sort:        &sp.PullRequestListOptions.Direction,
		OrderBy:     &orderBy,
	}
}

func (p *GitlabProvider) getUserFromBasicUser(i *gitlab.BasicUser) *models.User {
	if i == nil {
		return nil
	}
	id := int64(i.ID)
	return &models.User{
		ID:        &id,
		Name:      &i.Name,
		Login:     &i.Username, // TODO need to clarify
		AvatarURL: &i.AvatarURL,
		HTMLURL:   &i.WebURL, // TODO need to clarify
	}
}

func (p *GitlabProvider) getMilestone(i *gitlab.Milestone) *models.Milestone {
	if i == nil {
		return nil
	}
	id := int64(i.ID)
	dd := time.Time(*i.DueDate)
	return &models.Milestone{
		ID:          &id,
		Number:      &i.IID,
		Title:       &i.Title,
		Description: &i.Description,
		DueOn:       &dd,
		State:       &i.State,
		URL:         &i.WebURL, // TODO need to clarify
		CreatedAt:   i.CreatedAt,
		UpdatedAt:   i.UpdatedAt,
	}
}

func (p *GitlabProvider) getPullRequest(v *gitlab.MergeRequest) *models.PullRequest {
	id := int64(v.ID)
	m := &models.PullRequest{
		Assignee:  p.getUserFromBasicUser(v.Assignee),
		User:      p.getUserFromBasicUser(v.Author),
		Body:      &v.Description, // TODO need to clarify
		CreatedAt: v.CreatedAt,
		UpdatedAt: v.UpdatedAt,
		ClosedAt:  v.ClosedAt,
		URL:       &v.WebURL,
		Title:     &v.Title,
		State:     &v.State,
		ID:        &id,
		Number:    &v.IID,
		Milestone: p.getMilestone(v.Milestone),
	}
	return m
}

func (p *GitlabProvider) getPullRequests(i []*gitlab.MergeRequest) []*models.PullRequest {
	r := make([]*models.PullRequest, len(i))
	for k, v := range i {
		m := p.getPullRequest(v)
		r[k] = m
	}
	return r
}

func (p *GitlabProvider) PullRequestsList(sp models.SearchParams) (i []*models.PullRequest, r *models.Response, err error) {
	opt := p.getListProjectMergeRequestsOptions(sp)
	in, gr, err := p.client.MergeRequests.ListProjectMergeRequests(p.getProjectId(sp.Repo), opt)
	i = p.getPullRequests(in)
	r = p.getResponse(gr)
	return
}

func (p *GitlabProvider) PullRequestsGet(sp models.SearchParams) (i *models.PullRequest, r *models.Response, err error) {
	opt := &gitlab.GetMergeRequestsOptions{}
	in, gr, err := p.client.MergeRequests.GetMergeRequest(p.getProjectId(sp.Repo), sp.IssueNumber, opt)
	i = p.getPullRequest(in)
	r = p.getResponse(gr)
	return
}

func (p *GitlabProvider) getPullRequestComments(i []*gitlab.Note) []*models.PullRequestComment {
	r := make([]*models.PullRequestComment, len(i))
	for k, v := range i {
		id := int64(v.ID)
		m := &models.PullRequestComment{
			ID:        &id,
			Body:      &v.Body,
			CreatedAt: v.CreatedAt,
			UpdatedAt: v.UpdatedAt,
		}
		r[k] = m
	}
	return r
}

func (p *GitlabProvider) PullRequestsListComments(sp models.SearchParams) (i []*models.PullRequestComment, r *models.Response, err error) {
	opt := &gitlab.ListMergeRequestNotesOptions{
		ListOptions: p.getListOptions(sp.ListOptions),
	}
	in, gr, err := p.client.Notes.ListMergeRequestNotes(p.getProjectId(sp.Repo), sp.IssueNumber, opt)
	i = p.getPullRequestComments(in)
	r = p.getResponse(gr)
	return
}

func (p *GitlabProvider) getPullRequestReviews(i *gitlab.MergeRequestApprovals) []*models.PullRequestReview {
	r := make([]*models.PullRequestReview, len(i.ApprovedBy))
	state := "APPROVED"
	for k, v := range i.ApprovedBy {
		m := &models.PullRequestReview{
			User:  p.getUserFromBasicUser(v.User),
			State: &state,
		}
		r[k] = m
	}
	return r
}

func (p *GitlabProvider) PullRequestsListReviews(sp models.SearchParams) (i []*models.PullRequestReview, r *models.Response, err error) {
	// TODO need to clarify
	in, gr, err := p.client.MergeRequests.GetMergeRequestApprovals(p.getProjectId(sp.Repo), sp.IssueNumber)
	i = p.getPullRequestReviews(in)
	r = p.getResponse(gr)
	return
}

// https://gitlab.com/gitlab-org/gitlab-foss/-/issues/28342#note_23852124
func (p *GitlabProvider) getProjectId(repo models.Repo) string {
	var u string
	if repo.Group != "" {
		u = repo.Organization + "/" + repo.Group + "/" + repo.Project
	} else {
		u = repo.Organization + "/" + repo.Project
	}
	return u
}
