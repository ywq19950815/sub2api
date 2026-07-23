package admin

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
	"golang.org/x/sync/errgroup"
)

const livenessTaskRecentLimit = 12

type livenessTaskResult struct {
	AccountID int64  `json:"account_id"`
	Status    string `json:"status"`
	Error     string `json:"error,omitempty"`
}

// livenessTask keeps ephemeral UI progress; account errors remain persisted in the database.
type livenessTask struct {
	mu               sync.RWMutex
	ID               string               `json:"id"`
	State            string               `json:"state"`
	CreatedAt        time.Time            `json:"created_at"`
	FinishedAt       *time.Time           `json:"finished_at,omitempty"`
	Total            int                  `json:"total"`
	Completed        int                  `json:"completed"`
	Alive            int                  `json:"alive"`
	Dead             int                  `json:"dead"`
	UpdateFailed     int                  `json:"update_failed"`
	Deleted          int                  `json:"deleted"`
	CurrentAccountID int64                `json:"current_account_id,omitempty"`
	Recent           []livenessTaskResult `json:"recent"`
	failedAccountIDs []int64
}

type livenessTaskSnapshot struct {
	ID               string               `json:"id"`
	State            string               `json:"state"`
	CreatedAt        time.Time            `json:"created_at"`
	FinishedAt       *time.Time           `json:"finished_at,omitempty"`
	Total            int                  `json:"total"`
	Completed        int                  `json:"completed"`
	Alive            int                  `json:"alive"`
	Dead             int                  `json:"dead"`
	UpdateFailed     int                  `json:"update_failed"`
	Deleted          int                  `json:"deleted"`
	CurrentAccountID int64                `json:"current_account_id,omitempty"`
	Recent           []livenessTaskResult `json:"recent"`
}

func (t *livenessTask) snapshot() livenessTaskSnapshot {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return livenessTaskSnapshot{
		ID:               t.ID,
		State:            t.State,
		CreatedAt:        t.CreatedAt,
		FinishedAt:       t.FinishedAt,
		Total:            t.Total,
		Completed:        t.Completed,
		Alive:            t.Alive,
		Dead:             t.Dead,
		UpdateFailed:     t.UpdateFailed,
		Deleted:          t.Deleted,
		CurrentAccountID: t.CurrentAccountID,
		Recent:           append([]livenessTaskResult(nil), t.Recent...),
	}
}

func (t *livenessTask) record(result livenessTaskResult, updateFailed bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Completed++
	if result.Status == "alive" {
		t.Alive++
	} else {
		t.Dead++
		t.failedAccountIDs = append(t.failedAccountIDs, result.AccountID)
		if updateFailed {
			t.UpdateFailed++
		}
	}
	t.Recent = append([]livenessTaskResult{result}, t.Recent...)
	if len(t.Recent) > livenessTaskRecentLimit {
		t.Recent = t.Recent[:livenessTaskRecentLimit]
	}
}

type livenessTaskStore struct {
	mu    sync.RWMutex
	tasks map[string]*livenessTask
}

func newLivenessTaskStore() *livenessTaskStore {
	return &livenessTaskStore{tasks: make(map[string]*livenessTask)}
}

func (s *livenessTaskStore) create(total int) *livenessTask {
	task := &livenessTask{
		ID:        fmt.Sprintf("liveness-%d", time.Now().UnixNano()),
		State:     "running",
		CreatedAt: time.Now(),
		Total:     total,
	}
	s.mu.Lock()
	s.tasks[task.ID] = task
	s.mu.Unlock()
	return task
}

func (s *livenessTaskStore) get(id string) (*livenessTask, bool) {
	s.mu.RLock()
	task, ok := s.tasks[id]
	s.mu.RUnlock()
	return task, ok
}

func (h *AccountHandler) runLivenessTask(task *livenessTask, accountIDs []int64) {
	g, ctx := errgroup.WithContext(context.Background())
	g.SetLimit(10)
	for _, accountID := range accountIDs {
		accountID := accountID
		g.Go(func() error {
			task.mu.Lock()
			task.CurrentAccountID = accountID
			task.mu.Unlock()
			result, testErr := h.accountTestService.RunTestBackground(ctx, accountID, "")
			if testErr == nil && result != nil && result.Status == "success" {
				task.record(livenessTaskResult{AccountID: accountID, Status: "alive"}, false)
				return nil
			}
			errorMessage := "account liveness test failed"
			if testErr != nil {
				errorMessage = testErr.Error()
			} else if result != nil && result.ErrorMessage != "" {
				errorMessage = result.ErrorMessage
			}
			if len(errorMessage) > 1000 {
				errorMessage = errorMessage[:1000]
			}
			updateErr := h.adminService.SetAccountError(ctx, accountID, errorMessage)
			task.record(livenessTaskResult{AccountID: accountID, Status: "dead", Error: errorMessage}, updateErr != nil)
			return nil
		})
	}
	_ = g.Wait()
	now := time.Now()
	task.mu.Lock()
	task.State = "completed"
	task.FinishedAt = &now
	task.CurrentAccountID = 0
	task.mu.Unlock()
}

func (h *AccountHandler) GetBatchLivenessTask(c *gin.Context) {
	task, ok := h.livenessTasks.get(c.Param("taskID"))
	if !ok {
		response.NotFound(c, "Liveness task not found")
		return
	}
	response.Success(c, task.snapshot())
}

// DeleteBatchLivenessFailed deletes only accounts which failed this completed task.
func (h *AccountHandler) DeleteBatchLivenessFailed(c *gin.Context) {
	task, ok := h.livenessTasks.get(c.Param("taskID"))
	if !ok {
		response.NotFound(c, "Liveness task not found")
		return
	}
	task.mu.RLock()
	if task.State != "completed" {
		task.mu.RUnlock()
		response.BadRequest(c, "Liveness task is still running")
		return
	}
	ids := append([]int64(nil), task.failedAccountIDs...)
	task.mu.RUnlock()
	success, failed := 0, 0
	deleted := make(map[int64]struct{}, len(ids))
	for _, accountID := range ids {
		if err := h.adminService.DeleteAccount(c.Request.Context(), accountID); err != nil {
			failed++
			continue
		}
		success++
		deleted[accountID] = struct{}{}
	}
	task.mu.Lock()
	task.Deleted += success
	remaining := task.failedAccountIDs[:0]
	for _, id := range task.failedAccountIDs {
		if _, ok := deleted[id]; !ok {
			remaining = append(remaining, id)
		}
	}
	task.failedAccountIDs = remaining
	task.mu.Unlock()
	response.Success(c, gin.H{"total": len(ids), "deleted": success, "failed": failed})
}
