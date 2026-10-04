package workflow

import (
	"errors"
	"github.com/google/uuid"
	"nebulaiq/internal/domain"
	"nebulaiq/internal/providers"
	"nebulaiq/internal/storage/postgres"
	"strings"
	"time"
)

func (j *job) readVisualMenu(raw string) error {
	reader, ok := j.e.Model.(providers.MenuReader)
	if !ok || j.visionReads >= 2 || j.use.ModelCalls >= 7 || j.use.Fetches >= 20 {
		return errors.New("visual menu budget unavailable")
	}
	path := strings.ToLower(strings.Split(raw, "?")[0])
	supported := false
	for _, suffix := range []string{".pdf", ".png", ".jpg", ".jpeg", ".webp"} {
		supported = supported || strings.HasSuffix(path, suffix)
	}
	if !supported || !providers.SafeURL(raw) {
		return errors.New("not a menu file")
	}
	key := postgres.Hash("visual-menu-v2|" + j.e.Model.Name() + "|" + raw)
	for _, d := range j.docs {
		if d.URL == raw && d.Method == "vision_transcription" {
			j.tag(d.ID)
			return nil
		}
	}
	if !j.refresh {
		if d, hit, err := j.e.Store.Document(j.ctx, key); err == nil && hit {
			d.Historical = d.Historical || historicalURL(d.URL)
			j.docs = append(j.docs, d)
			j.tag(d.ID)
			j.use.CacheHits++
			return nil
		}
	}
	if j.visualAttempts == nil {
		j.visualAttempts = map[string]bool{}
	}
	if j.visualAttempts[raw] {
		return errors.New("visual menu already attempted")
	}
	j.visualAttempts[raw] = true
	j.visionReads++
	j.use.Fetches++
	_ = j.event("menu.transcribing", "Reading menu file layout and dish prices: "+raw)
	text, use, err := reader.ReadMenu(j.ctx, raw)
	j.use.ModelCalls += use.ModelCalls
	j.use.InputTokens += use.InputTokens
	j.use.OutputTokens += use.OutputTokens
	j.use.UsageKnown = j.use.UsageKnown || use.UsageKnown
	if err != nil {
		j.limitations = append(j.limitations, "Visual menu could not be read: "+raw)
		return err
	}
	if len(strings.TrimSpace(text)) < 80 {
		return errors.New("menu transcription contains insufficient readable information")
	}
	now := time.Now().UTC()
	d := domain.Document{ID: uuid.NewString(), URL: raw, Title: j.activeCandidate + " menu transcription", Text: text, Kind: "menu", Method: "vision_transcription", Historical: historicalURL(raw), FetchedAt: now, ExpiresAt: now.Add(postgres.DocTTL("menu")), Hash: postgres.Hash(text)}
	if err = j.e.Store.PutDocument(j.ctx, key, d); err != nil {
		return err
	}
	d.Historical = d.Historical || historicalURL(d.URL)
	j.docs = append(j.docs, d)
	j.tag(d.ID)
	_ = j.event("menu.transcribed", "Retained a menu transcription; verify uncertain details against the original file.")
	return nil
}
