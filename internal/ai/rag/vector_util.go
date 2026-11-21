package rag

// cosineSimilarity 计算两个向量的余弦相似度。
// 向量长度不同或为空时返回 0。
func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dotProduct / (sqrt(normA) * sqrt(normB))
}

// sqrt 使用牛顿迭代法计算平方根。
func sqrt(x float64) float64 {
	if x <= 0 {
		return 0
	}
	z := x / 2
	for i := 0; i < 20; i++ {
		z = (z + x/z) / 2
	}
	return z
}
