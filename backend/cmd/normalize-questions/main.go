// One-shot tool to re-normalize content_normalized for existing rows
// using the canonical normalizeQuestionContent() function from the
// app package. Run after deploying migration 000019.
//
// Usage:  go run ./cmd/normalize-questions
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"

	"morfoschools/backend/internal/app"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is required")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	rows, err := db.QueryContext(context.Background(),
		`SELECT id::text, content FROM exam_questions WHERE content IS NOT NULL`)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	type item struct{ id, content string }
	var items []item
	for rows.Next() {
		var it item
		if err := rows.Scan(&it.id, &it.content); err == nil {
			items = append(items, it)
		}
	}
	rows.Close()

	updated := 0
	for _, it := range items {
		norm := app.NormalizeQuestionContent(it.content)
		hash := app.HashContent(it.content)
		_, err := db.ExecContext(context.Background(),
			`UPDATE exam_questions SET content_normalized = $1, content_hash = $2 WHERE id = $3`,
			norm, hash, it.id)
		if err != nil {
			log.Printf("warn: update %s: %v", it.id, err)
			continue
		}
		updated++
	}
	fmt.Printf("updated %d / %d rows\n", updated, len(items))
}
