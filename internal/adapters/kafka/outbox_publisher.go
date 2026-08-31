package kafka

import (
	"context"
	"database/sql"
	"log"
	"time"

	"github.com/segmentio/kafka-go"
)

// OutboxPublisher polls the outbox table for unpublished rows, sends
// each to Kafka, and marks it published — all inside one Postgres
// transaction per batch, using SELECT ... FOR UPDATE SKIP LOCKED so
// multiple publisher instances could run concurrently without double
// -claiming the same row (only one publisher runs today, but this
// keeps the door open to running more for throughput later).
//
// Delivery here is at-least-once, not exactly-once: if WriteMessages
// succeeds for some rows in a batch but a later row in the same batch
// fails, the whole transaction rolls back and every row in that batch
// — including the ones already sent to Kafka — gets retried on the
// next tick. Consumers of these topics must tolerate an occasional
// duplicate message. This is the standard, accepted trade-off for the
// transactional outbox pattern.
type OutboxPublisher struct {
	db       *sql.DB
	writer   *kafka.Writer
	interval time.Duration
	batch    int
	cancel   context.CancelFunc
	done     chan struct{}
}

func NewOutboxPublisher(db *sql.DB, writer *kafka.Writer) *OutboxPublisher {
	return &OutboxPublisher{
		db:       db,
		writer:   writer,
		interval: 500 * time.Millisecond,
		batch:    50,
		done:     make(chan struct{}),
	}
}

func (p *OutboxPublisher) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	p.cancel = cancel
	go p.loop(ctx)
}

func (p *OutboxPublisher) loop(ctx context.Context) {
	defer close(p.done)
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := p.publishBatch(ctx); err != nil {
				log.Printf("outbox publish batch error: %v", err)
			}
		}
	}
}

type outboxRow struct {
	ID      string
	Topic   string
	Key     string
	Payload []byte
}

func (p *OutboxPublisher) publishBatch(ctx context.Context) error {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, `
		SELECT id, topic, key, payload FROM outbox
		WHERE published_at IS NULL
		ORDER BY created_at
		LIMIT $1
		FOR UPDATE SKIP LOCKED`,
		p.batch,
	)
	if err != nil {
		return err
	}

	var batch []outboxRow
	for rows.Next() {
		var r outboxRow
		if err := rows.Scan(&r.ID, &r.Topic, &r.Key, &r.Payload); err != nil {
			rows.Close()
			return err
		}
		batch = append(batch, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	if len(batch) == 0 {
		return tx.Commit()
	}

	for _, r := range batch {
		if err := p.writer.WriteMessages(ctx, kafka.Message{
			Topic: r.Topic, Key: []byte(r.Key), Value: r.Payload,
		}); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE outbox SET published_at = now() WHERE id = $1`, r.ID); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (p *OutboxPublisher) Shutdown() {
	if p.cancel != nil {
		p.cancel()
	}
	<-p.done
}
