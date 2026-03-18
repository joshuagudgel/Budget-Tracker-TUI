package ml

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"budget-tracker-tui/internal/types"
)

// CategoryPrediction represents an ML prediction result
type CategoryPrediction struct {
	CategoryId   int64   `json:"categoryId"`
	Confidence   float64 `json:"confidence"`   // 0.0 - 1.0
	ReasonCode   string  `json:"reasonCode"`   // "exact_match", "similarity", "fallback"
	SimilarityTo string  `json:"similarityTo"` // description of most similar historical transaction
}

// TrainingExample represents a labeled example for training
type TrainingExample struct {
	Description string    `json:"description"`
	Amount      float64   `json:"amount"`
	CategoryId  int64     `json:"categoryId"`
	Timestamp   time.Time `json:"timestamp"`
	Source      string    `json:"source"` // "user_edit", "split", etc.
}

// EmbeddingsCategorizer uses simple text similarity for transaction categorization
type EmbeddingsCategorizer struct {
	trainingExamples    []TrainingExample
	availableCategories []types.Category
	defaultCategoryId   int64

	// Configuration
	minConfidenceThreshold float64 // predictions below this trigger user review
	exactMatchBonus        float64 // bonus for exact description matches
	amountSimilarityWeight float64 // weight for amount-based similarity
}

// NewEmbeddingsCategorizer creates a new ML categorization service
func NewEmbeddingsCategorizer(defaultCategoryId int64) *EmbeddingsCategorizer {
	return &EmbeddingsCategorizer{
		trainingExamples:       make([]TrainingExample, 0),
		availableCategories:    make([]types.Category, 0),
		defaultCategoryId:      defaultCategoryId,
		minConfidenceThreshold: 0.7, // configurable later
		exactMatchBonus:        0.3,
		amountSimilarityWeight: 0.2,
	}
}

// Train loads training data from audit events and categories
func (ec *EmbeddingsCategorizer) Train(auditEvents []types.TransactionAuditEvent, categories []types.Category) error {
	ec.availableCategories = categories
	ec.trainingExamples = make([]TrainingExample, 0)

	// Extract training examples from user edit audit events
	for _, event := range auditEvents {
		// Only use user edits that changed categories as high-quality labels
		if event.ActionType == types.ActionTypeEdit &&
			event.Source == types.SourceUser &&
			event.ModificationReason != nil &&
			*event.ModificationReason == types.ModReasonCategory {

			example := TrainingExample{
				Description: event.DescriptionFingerprint,
				CategoryId:  event.CategoryAssigned, // The corrected category
				Timestamp:   event.Timestamp,
				Source:      "user_edit",
			}

			// TODO: Extract amount from PreEditSnapshot JSON if needed for amount-based similarity
			// For now, focus on description-based categorization
			example.Amount = 0.0

			ec.trainingExamples = append(ec.trainingExamples, example)
		}
	}

	fmt.Printf("[ML] Trained categorizer with %d examples from audit events\n", len(ec.trainingExamples))
	return nil
}

// PredictCategory predicts the category for a new transaction using embeddings similarity
func (ec *EmbeddingsCategorizer) PredictCategory(description string, amount float64) CategoryPrediction {
	if len(ec.trainingExamples) == 0 {
		// No training data - return default category with low confidence
		return CategoryPrediction{
			CategoryId:   ec.defaultCategoryId,
			Confidence:   0.1,
			ReasonCode:   "fallback",
			SimilarityTo: "no training data available",
		}
	}

	// Normalize description for comparison
	normalizedDesc := ec.normalizeDescription(description)

	// Check for exact matches first
	for _, example := range ec.trainingExamples {
		if ec.normalizeDescription(example.Description) == normalizedDesc {
			return CategoryPrediction{
				CategoryId:   example.CategoryId,
				Confidence:   0.95 + ec.exactMatchBonus, // High confidence for exact matches
				ReasonCode:   "exact_match",
				SimilarityTo: example.Description,
			}
		}
	}

	// Find best similarity match
	bestMatch := ec.findBestSimilarityMatch(normalizedDesc, amount)
	return bestMatch
}

// IsHighConfidence checks if the prediction confidence is above the threshold
func (ec *EmbeddingsCategorizer) IsHighConfidence(prediction CategoryPrediction) bool {
	return prediction.Confidence >= ec.minConfidenceThreshold
}

// GetStats returns training statistics for debugging
func (ec *EmbeddingsCategorizer) GetStats() map[string]interface{} {
	categoryCount := make(map[int64]int)
	for _, example := range ec.trainingExamples {
		categoryCount[example.CategoryId]++
	}

	return map[string]interface{}{
		"total_examples":           len(ec.trainingExamples),
		"categories_with_examples": len(categoryCount),
		"min_confidence_threshold": ec.minConfidenceThreshold,
		"category_distribution":    categoryCount,
	}
}

// normalizeDescription cleans and normalizes transaction descriptions for comparison
func (ec *EmbeddingsCategorizer) normalizeDescription(desc string) string {
	// Convert to lowercase
	normalized := strings.ToLower(desc)

	// Remove extra whitespace
	normalized = strings.TrimSpace(normalized)
	normalized = strings.Join(strings.Fields(normalized), " ")

	// Remove common prefixes/suffixes that add noise
	prefixes := []string{"pos ", "purchase ", "payment ", "debit card ", "online "}
	suffixes := []string{" purchase", " payment", " online", " pos"}

	for _, prefix := range prefixes {
		if strings.HasPrefix(normalized, prefix) {
			normalized = strings.TrimPrefix(normalized, prefix)
			break
		}
	}

	for _, suffix := range suffixes {
		if strings.HasSuffix(normalized, suffix) {
			normalized = strings.TrimSuffix(normalized, suffix)
			break
		}
	}

	return normalized
}

// findBestSimilarityMatch finds the most similar training example using text similarity
func (ec *EmbeddingsCategorizer) findBestSimilarityMatch(normalizedDesc string, amount float64) CategoryPrediction {
	type scoredExample struct {
		example TrainingExample
		score   float64
	}

	scores := make([]scoredExample, 0, len(ec.trainingExamples))

	for _, example := range ec.trainingExamples {
		normalizedExample := ec.normalizeDescription(example.Description)
		textSimilarity := ec.calculateTextSimilarity(normalizedDesc, normalizedExample)
		amountSimilarity := ec.calculateAmountSimilarity(amount, example.Amount)

		// Combine text and amount similarity (weighted toward text)
		totalScore := textSimilarity + (ec.amountSimilarityWeight * amountSimilarity)

		scores = append(scores, scoredExample{
			example: example,
			score:   totalScore,
		})
	}

	// Sort by score descending
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].score > scores[j].score
	})

	if len(scores) == 0 {
		// Fallback to default
		return CategoryPrediction{
			CategoryId:   ec.defaultCategoryId,
			Confidence:   0.1,
			ReasonCode:   "fallback",
			SimilarityTo: "no similar examples found",
		}
	}

	bestMatch := scores[0]
	return CategoryPrediction{
		CategoryId:   bestMatch.example.CategoryId,
		Confidence:   math.Min(bestMatch.score, 0.99), // Cap at 0.99 to distinguish from exact matches
		ReasonCode:   "similarity",
		SimilarityTo: bestMatch.example.Description,
	}
}

// calculateTextSimilarity uses simple word overlap similarity (Jaccard-like)
func (ec *EmbeddingsCategorizer) calculateTextSimilarity(desc1, desc2 string) float64 {
	words1 := strings.Fields(desc1)
	words2 := strings.Fields(desc2)

	if len(words1) == 0 && len(words2) == 0 {
		return 1.0
	}
	if len(words1) == 0 || len(words2) == 0 {
		return 0.0
	}

	// Create word sets
	set1 := make(map[string]bool)
	set2 := make(map[string]bool)

	for _, word := range words1 {
		set1[word] = true
	}
	for _, word := range words2 {
		set2[word] = true
	}

	// Calculate intersection
	intersection := 0
	for word := range set1 {
		if set2[word] {
			intersection++
		}
	}

	// Calculate union
	union := len(set1) + len(set2) - intersection

	if union == 0 {
		return 0.0
	}

	// Jaccard similarity
	return float64(intersection) / float64(union)
}

// calculateAmountSimilarity calculates similarity based on transaction amounts
func (ec *EmbeddingsCategorizer) calculateAmountSimilarity(amount1, amount2 float64) float64 {
	if amount1 == 0.0 && amount2 == 0.0 {
		return 1.0
	}
	if amount1 == 0.0 || amount2 == 0.0 {
		return 0.0 // Skip amount comparison if either is zero (training data limitation)
	}

	// Calculate relative difference
	diff := math.Abs(amount1 - amount2)
	avg := (math.Abs(amount1) + math.Abs(amount2)) / 2.0

	if avg == 0.0 {
		return 1.0
	}

	// Convert to similarity (inverse of relative difference)
	relativeDiff := diff / avg
	similarity := math.Max(0.0, 1.0-relativeDiff)

	return similarity
}
