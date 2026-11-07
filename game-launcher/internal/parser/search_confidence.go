package parser

// ConfidentSearchURL deliberately keeps the existing global Best threshold.
// An exact title alone is insufficient (including equal-score cross-site ties).
// Conflicting marked winners or known authors require a human choice.
func ConfidentSearchURL(query string, result SearchResult) string {
	name := normTitle(query)
	if name == "" || len(result.Hits) == 0 {
		return ""
	}
	best := result.Hits[0]
	for _, hit := range result.Hits {
		if hit.Score > best.Score {
			best = hit
		}
	}
	if !best.Best || best.URL == "" || normTitle(best.Title) != name {
		return ""
	}
	for _, hit := range result.Hits {
		if hit.URL == best.URL {
			continue
		}
		if hit.Best && normTitle(hit.Title) != name {
			return ""
		}
		if (hit.Best || normTitle(hit.Title) == name) && best.Author != "" && hit.Author != "" && normTitle(best.Author) != normTitle(hit.Author) {
			return ""
		}
		// Distinct records on one source with the same title are not interchangeable.
		if hit.Best && hit.Source == best.Source {
			return ""
		}
	}
	return best.URL
}
