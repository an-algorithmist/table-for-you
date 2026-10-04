package workflow

import (
	"nebulaiq/internal/domain"
	"testing"
)

func TestMenuSelectionPrefersLocationAndUndatedRestaurantSource(t *testing.T) {
	hits := []domain.SearchHit{{URL: "https://blog.example/2018/03/22/green-spot", Title: "The Green Spot", Content: "Barcelona Spain"}, {URL: "https://tripadvisor.example/review", Title: "The Green Spot", Content: "Barcelona Spain"}, {URL: "https://group.example/restaurants/green-spot/menu", Title: "The Green Spot", Content: "Barcelona Spain lunch menu"}}
	got := prioritizeMenus(hits, domain.Candidate{Name: "The Green Spot"}, domain.Requirements{City: "Barcelona", Country: "Spain"})
	if got[0].URL != hits[2].URL {
		t.Fatal("dated blog won over restaurant menu source")
	}
	if !historicalURL(hits[0].URL) {
		t.Fatal("dated source not flagged historical")
	}
}

func TestMenuFilenameYearAndDeliverySource(t *testing.T) {
	if !historicalURL("https://restaurant.example/uploads/2025/01/Aguaribay_Menu_2023_Espanol_01.pdf") {
		t.Fatal("old filename hidden by newer upload folder")
	}
	if historicalURL("https://restaurant.example/menu.pdf") {
		t.Fatal("undated menu falsely historical")
	}
	if !deliveryURL("https://postmates.com/store/divyas-kitchen/menu") {
		t.Fatal("delivery source not recognized")
	}
	if deliveryURL("https://divyaskitchen.example/menu") {
		t.Fatal("restaurant source rejected")
	}
	j := job{docs: []domain.Document{{ID: "d", URL: "https://postmates.com/store/divyas-kitchen/menu", Kind: "menu", Text: "Divya's Kitchen AED 15"}}, docCandidates: map[string]map[string]bool{"d": {"Divya's Kitchen": true}}}
	if len(j.candidateDocs("Divya's Kitchen")) != 0 {
		t.Fatal("delivery menu entered regular-menu extraction")
	}
}
