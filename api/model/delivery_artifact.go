package model

// ArtifactShareRequest copies a retained template artifact without a runtime component or snapshot.
type ArtifactShareRequest struct {
	ServiceKey  string `json:"service_key"`
	Version     string `json:"version"`
	SourceImage string `json:"source_image"`
	SourceSlug  string `json:"source_slug"`
	EventID     string `json:"event_id"`
	Actor       string `json:"actor"`
	SourceAuth  struct {
		User     string `json:"user"`
		Password string `json:"password"`
	} `json:"source_auth"`
	ImageInfo struct {
		HubURL      string `json:"hub_url"`
		HubUser     string `json:"hub_user"`
		HubPassword string `json:"hub_password"`
		Namespace   string `json:"namespace"`
	} `json:"image_info"`
	SlugInfo struct {
		Namespace   string `json:"namespace"`
		FTPHost     string `json:"ftp_host"`
		FTPPort     string `json:"ftp_port"`
		FTPUser     string `json:"ftp_username"`
		FTPPassword string `json:"ftp_password"`
	} `json:"slug_info"`
}
