package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log"
	"os"
	"time"

	"slopyard/internal/domain"
	"slopyard/internal/store"
)

type seedReport struct {
	input    string
	kind     string
	category string
	notes    string
	reporter string
}

func main() {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	pg, err := store.NewPostgres(ctx, databaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pg.Close()

	reports := []seedReport{
		{input: "https://www.example.com/ai-listicle", kind: store.ReportSlop, category: "SEO_SPAM", notes: "Seed report", reporter: "seed-1"},
		{input: "example.com", kind: store.ReportSlop, category: "LOW_EFFORT", notes: "Seed report", reporter: "seed-2"},
		{input: "blog.example.com/research", kind: store.ReportNotSlop, category: "HUMAN_AUTHORED", notes: "Seed report", reporter: "seed-3"},
		{input: "contentfarm.dev", kind: store.ReportSlop, category: "CONTENT_FARM", notes: "Seed report", reporter: "seed-4"},
		{input: "contentfarm.dev/best-products", kind: store.ReportSlop, category: "FAKE_REVIEWS", notes: "Seed report", reporter: "seed-5"},
		{input: "useful-ai.org", kind: store.ReportNotSlop, category: "USEFUL_AI", notes: "Seed report", reporter: "seed-6"},
		{input: "https://www.clear-disclosure.net/article", kind: store.ReportNotSlop, category: "CLEAR_DISCLOSURE", notes: "Seed report", reporter: "seed-7"},
	}

	for _, report := range reports {
		normalized, err := domain.NormalizeHost(report.input)
		if err != nil {
			log.Fatalf("normalize %q: %v", report.input, err)
		}
		if _, err := pg.CreateReport(ctx, store.ReportInput{
			SiteHost:          normalized.Host,
			RegistrableDomain: normalized.RegistrableDomain,
			SubmittedInput:    report.input,
			Type:              report.kind,
			Category:          report.category,
			Notes:             report.notes,
			FingerprintHash:   fingerprint(report.reporter),
		}); err != nil {
			log.Fatalf("insert seed report for %q: %v", report.input, err)
		}
	}

	log.Printf("seeded %d reports", len(reports))
}

func fingerprint(value string) string {
	sum := sha256.Sum256([]byte("seed:" + value))
	return hex.EncodeToString(sum[:])
}
