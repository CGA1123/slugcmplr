package compilesvc_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/cga1123/slugcmplr/proto/compileworker"
	"github.com/cga1123/slugcmplr/queue"
	"github.com/cga1123/slugcmplr/services/compilesvc"
	"github.com/cga1123/slugcmplr/services/compilesvc/store"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type dispatcher struct {
	tokens []string
}

func (d *dispatcher) Dispatch(_ context.Context, token string) error {
	d.tokens = append(d.tokens, token)

	return nil
}

type repo struct{}

func (r *repo) DownloadURL(_ context.Context, _, _, _ string) (string, error) {
	return "", errors.New("nyi")
}

func (r *repo) ConfigFile(_ context.Context, _, _, _ string) (*compilesvc.RepoConfig, error) {
	return &compilesvc.RepoConfig{Targets: []string{"foo", "bar"}}, nil
}

type querier struct {
	tokens       []store.CreateTokenParams
	compilations []store.CreateParams
}

func (q *querier) Create(_ context.Context, arg store.CreateParams) error {
	q.compilations = append(q.compilations, arg)
	return nil
}

func (q *querier) CreateToken(_ context.Context, arg store.CreateTokenParams) error {
	q.tokens = append(q.tokens, arg)
	return nil
}

func (q *querier) ExpireToken(_ context.Context, _ string) error {
	return errors.New("nyi")
}

func (q *querier) FetchWithToken(_ context.Context, _ string) (store.Compilation, error) {
	return store.Compilation{}, errors.New("nyi")
}

func Test_Triggers(t *testing.T) {
	t.Parallel()

	m := mux.NewRouter()
	q := make(queue.InMemory, 0)
	d := &dispatcher{}
	r := &repo{}
	s := &querier{}

	compilesvc.Work(m, &q, s, r, d)
	client, closer := fakeServer(func(w http.ResponseWriter, r *http.Request) {
		m.ServeHTTP(w, r)
	})
	defer closer()

	c := compileworker.NewCompileJSONClient("https://foo.com", client)
	_, err := c.TriggerForRepository(context.Background(), &compileworker.RepositoryInfo{
		EventId:    "eventid",
		CommitSha:  "abcd1234",
		TreeSha:    "1234abcd",
		Owner:      "CGA1123",
		Repository: "slugcmplr",
		Ref:        "/refs/heads/main",
	})
	require.NoError(t, err)

	assert.Equal(t, 2, len(q))
	assert.Equal(t, 0, len(s.compilations))
	assert.Equal(t, 0, len(s.tokens))

	w := queue.TwirpWorker(m)
	require.NoError(t, q.Deq(context.Background(), "default", w))
	require.NoError(t, q.Deq(context.Background(), "default", w))

	assert.Equal(t, 2, len(s.compilations))
	assert.Equal(t, 2, len(s.tokens))

	comps := make([]string, 0)
	toks := make([]string, 0)
	for i := 0; i < 2; i++ {
		c, t := s.compilations[i], s.tokens[i]
		comps = append(comps, fmt.Sprintf("%v-%v", c.EventID, c.Target))
		toks = append(toks, fmt.Sprintf("%v-%v", t.EventID, t.Target))
	}
	assert.ElementsMatch(t, []string{"eventid-foo", "eventid-bar"}, comps)
	assert.ElementsMatch(t, []string{"eventid-foo", "eventid-bar"}, toks)

	tokens := make([]string, 0)
	for _, t := range s.tokens {
		tokens = append(tokens, t.Token)
	}
	assert.Equal(t, 2, len(d.tokens))
	assert.ElementsMatch(t, tokens, d.tokens)
}
