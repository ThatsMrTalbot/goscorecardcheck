package scorecard

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
)

type Score struct {
	Score  float64 `json:"score"`
	Checks []Check `json:"checks"`
}

type Check struct {
	Name          string        `json:"name"`
	Reason        string        `json:"reason"`
	Score         float64       `json:"score"`
	Details       []string      `json:"details"`
	Documentation Documentation `json:"documentation"`
}

type Documentation struct {
	Short string `json:"short"`
	URL   string `json:"url"`
}

type Getter struct {
	scorecards sync.Map
}

func (s *Getter) Get(ctx context.Context, dep Dependency) (Score, error) {
	type fn = func() (Score, error)

	getter, _ := s.scorecards.LoadOrStore(dep, sync.OnceValues(func() (Score, error) {
		return getScoreForDependency(ctx, dep)
	}))

	return getter.(fn)()
}

func (s *Getter) GetForImportPath(ctx context.Context, importPath string) (Dependency, Score, error) {
	dep, err := ParseDependencyForImportPath(importPath)
	if err != nil {
		return Dependency{}, Score{}, err
	}

	score, err := s.Get(ctx, dep)
	return dep, score, err
}

func getScoreForDependency(ctx context.Context, dep Dependency) (Score, error) {
	const api = "https://api.securityscorecards.dev"
	url := fmt.Sprintf("%s/projects/%s/%s/%s?commit=%s", api, dep.Platform, dep.Org, dep.Repo, dep.Commit)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Score{}, fmt.Errorf("could not create api request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Score{}, fmt.Errorf("error performing scorecard api request: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return Score{}, fmt.Errorf("no scorecard for import %s", dep.Root)
	}

	if resp.StatusCode != 200 {
		return Score{}, fmt.Errorf("scorecard api gave status code %d", resp.StatusCode)
	}

	var result Score
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return Score{}, fmt.Errorf("failed to unmarshal scorecard response: %w", err)
	}

	return result, nil
}
