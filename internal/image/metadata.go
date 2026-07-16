package image

type Metadata struct {
	Name    string   `json:"name"`
	Command []string `json:"command"`
}

type Image struct {
	RootFS   string
	Metadata Metadata
}
