package main

type Track struct {
	Title   string   `json:"title"`
	Album   string   `json:"album"`
	Artists []string `json:"artists"`
}
