package app

type Design struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Primary     string `json:"primary"`
	Secondary   string `json:"secondary"`
	Accent      string `json:"accent"`
	Background  string `json:"background"`
	Text        string `json:"text"`
	Description string `json:"description"`
}

var Designs = []Design{
	{
		ID:          "classic",
		Name:        "Classic White",
		Primary:     "#0a0a0a",
		Secondary:   "#5a5249",
		Accent:      "#c8a26a",
		Background:  "#f6f4ee",
		Text:        "#0a0a0a",
		Description: "Editorial cream — modern minimal",
	},
	{
		ID:          "romantic",
		Name:        "Romantic Rose",
		Primary:     "#b76e79",
		Secondary:   "#e8c4c8",
		Accent:      "#8b4a52",
		Background:  "#fdf5f6",
		Text:        "#4a2c2e",
		Description: "Soft and romantic",
	},
	{
		ID:          "boho",
		Name:        "Boho Earth",
		Primary:     "#6b705c",
		Secondary:   "#b7b7a4",
		Accent:      "#cb997e",
		Background:  "#f5f0eb",
		Text:        "#3d3d3d",
		Description: "Natural and warm",
	},
	{
		ID:          "modern",
		Name:        "Modern Dark",
		Primary:     "#1a1a2e",
		Secondary:   "#16213e",
		Accent:      "#e94560",
		Background:  "#0f3460",
		Text:        "#e0e0e0",
		Description: "Bold and stylish",
	},
	{
		ID:          "garden",
		Name:        "Garden Party",
		Primary:     "#2d6a4f",
		Secondary:   "#52b788",
		Accent:      "#f4a261",
		Background:  "#f0fff4",
		Text:        "#1b4332",
		Description: "Fresh and vibrant",
	},
}

func GetDesignByID(id string) *Design {
	for i := range Designs {
		if Designs[i].ID == id {
			return &Designs[i]
		}
	}
	return nil
}