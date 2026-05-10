package mcp

import (
	"context"
	"errors"
	"fmt"

	mcppkg "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const defaultWriteQueueSize = 32

var ErrWriteQueueFull = errors.New("memory write queue is full; retry shortly")

type WriteQueue struct {
	jobs chan WriteJob
}

type WriteJob struct {
	ctx    context.Context
	run    func(context.Context) (*mcppkg.CallToolResult, error)
	result chan WriteResult
}

type WriteResult struct {
	result *mcppkg.CallToolResult
	err    error
}

func NewWriteQueue(size int) *WriteQueue {
	if size <= 0 {
		size = defaultWriteQueueSize
	}
	q := &WriteQueue{jobs: make(chan WriteJob, size)}
	go q.loop()
	return q
}

func (q *WriteQueue) loop() {
	for job := range q.jobs {
		if err := job.ctx.Err(); err != nil {
			job.result <- WriteResult{err: err}
			continue
		}

		result, err := runWriteJob(job)
		job.result <- WriteResult{result: result, err: err}
	}
}

func (q *WriteQueue) Do(ctx context.Context, run func(context.Context) (*mcppkg.CallToolResult, error)) (*mcppkg.CallToolResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	job := WriteJob{
		ctx:    ctx,
		run:    run,
		result: make(chan WriteResult, 1),
	}

	select {
	case q.jobs <- job:
		// Enqueued.
	default:
		return nil, ErrWriteQueueFull
	}

	// The worker owns the post-enqueue cancellation decision. Returning directly
	// on ctx.Done() here can race with the worker starting the job: the caller may
	// see cancellation while the handler is about to mutate SQLite. Waiting for the
	// worker's result makes the outcome deterministic: queued canceled jobs are
	// skipped by the worker before start, while started jobs finish and return the
	// handler result.
	res := <-job.result
	return res.result, res.err
}

func runWriteJob(job WriteJob) (result *mcppkg.CallToolResult, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			result = nil
			err = fmt.Errorf("mcp write handler panic: %v", recovered)
		}
	}()

	return job.run(job.ctx)
}

func QueuedWriteHandler(q *WriteQueue, h server.ToolHandlerFunc) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcppkg.CallToolRequest) (*mcppkg.CallToolResult, error) {
		result, err := q.Do(ctx, func(runCtx context.Context) (*mcppkg.CallToolResult, error) {
			return h(runCtx, req)
		})
		if err == nil {
			return result, nil
		}
		if errors.Is(err, ErrWriteQueueFull) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return mcppkg.NewToolResultError(fmt.Sprintf("MCP write queue error: %s", err)), nil
		}
		return nil, err
	}
}
