// Package thumbnailqueue holds a bounded worker pool that generates
// photo thumbnails off the request path.
package thumbnailqueue

import (
	"log"
	"sync"

	domainphoto "yadegar/internal/domain/photo"
)

type Job struct {
	URL string
}

// Pool implements domainphoto.ThumbnailQueue with a fixed number of
// worker goroutines and a bounded channel. Enqueue blocks once the
// channel is full, instead of spawning unbounded goroutines.
type Pool struct {
	jobs    chan Job
	storage domainphoto.Storage
	wg      sync.WaitGroup
}

func NewPool(storage domainphoto.Storage, workers, queueSize int) *Pool {
	p := &Pool{jobs: make(chan Job, queueSize), storage: storage}
	for i := 0; i < workers; i++ {
		p.wg.Add(1)
		go p.worker()
	}
	return p
}

func (p *Pool) worker() {
	defer p.wg.Done()
	for job := range p.jobs {
		if _, err := p.storage.EnsureThumbnail(job.URL); err != nil {
			log.Printf("thumbnail generation failed for %s: %v", job.URL, err)
		}
	}
}

// Enqueue is non-blocking up to queueSize, then blocks. Never drops a
// job silently.
func (p *Pool) Enqueue(url string) {
	p.jobs <- Job{URL: url}
}

// Shutdown closes the queue and waits for in-flight jobs to finish.
// Call this from main.go during graceful shutdown (see step A3).
func (p *Pool) Shutdown() {
	close(p.jobs)
	p.wg.Wait()
}

// QueueDepth returns how many jobs are currently waiting in the queue.
// Used by the Prometheus gauge in telemetry.RegisterThumbnailQueueDepth.
func (p *Pool) QueueDepth() int {
	return len(p.jobs)
}
