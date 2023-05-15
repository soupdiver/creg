package adguardhome

type RewriteListResponse []RewriteListResponseItem

type RewriteListResponseItem struct {
	Domain string `json:"domain"`
	Answer string `json:"answer"`
}
