package claude

import (
	"context"
	"sync"

	"github.com/godeps/claude-agent-sdk-go/types"
)

// AgentResult holds the result of a single agent query.
type AgentResult struct {
	Prompt   string
	Messages []types.Message
	Error    error
}

// AgentPool manages a pool of concurrent Claude queries.
type AgentPool struct {
	options     *types.ClaudeAgentOptions
	concurrency int
}

// NewAgentPool creates a pool with the given concurrency limit and shared options.
func NewAgentPool(concurrency int, opts *types.ClaudeAgentOptions) *AgentPool {
	if concurrency <= 0 {
		concurrency = 1
	}
	return &AgentPool{
		options:     opts,
		concurrency: concurrency,
	}
}

// FanOut sends multiple prompts concurrently and collects all results.
// Results are returned in the same order as prompts.
func (p *AgentPool) FanOut(ctx context.Context, prompts []string) []AgentResult {
	results := make([]AgentResult, len(prompts))
	sem := make(chan struct{}, p.concurrency)
	var wg sync.WaitGroup

	for i, prompt := range prompts {
		wg.Add(1)
		go func(idx int, pr string) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				results[idx] = AgentResult{Prompt: pr, Error: ctx.Err()}
				return
			}

			ch, err := Query(ctx, pr, p.options)
			if err != nil {
				results[idx] = AgentResult{Prompt: pr, Error: err}
				return
			}

			var msgs []types.Message
			for msg := range ch {
				msgs = append(msgs, msg)
			}
			results[idx] = AgentResult{Prompt: pr, Messages: msgs}
		}(i, prompt)
	}

	wg.Wait()
	return results
}

// MapFunc transforms an item into a prompt for Claude.
type MapFunc func(item string) string

// ReduceFunc combines multiple AgentResults into a single summary prompt.
type ReduceFunc func(results []AgentResult) string

// MapReduce splits work across agents, then reduces results with another query.
func (p *AgentPool) MapReduce(ctx context.Context, items []string, mapFn MapFunc, reduceFn ReduceFunc) (*AgentResult, error) {
	prompts := make([]string, len(items))
	for i, item := range items {
		prompts[i] = mapFn(item)
	}

	mapResults := p.FanOut(ctx, prompts)

	reducePrompt := reduceFn(mapResults)

	ch, err := Query(ctx, reducePrompt, p.options)
	if err != nil {
		return nil, err
	}

	var msgs []types.Message
	for msg := range ch {
		msgs = append(msgs, msg)
	}

	return &AgentResult{
		Prompt:   reducePrompt,
		Messages: msgs,
	}, nil
}
