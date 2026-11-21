package cache

import "fmt"

const (
	// globalPrefix is prepended to all cache keys to avoid collisions.
	globalPrefix = "citebase:"

	// User cache keys
	KeyUserPrefix = globalPrefix + "user:"       // user:{id}
	KeyUserByName = globalPrefix + "user:name:"  // user:name:{username}
	KeyUserToken  = globalPrefix + "user:token:" // user:token:{token}

	// Document cache keys
	KeyDocumentPrefix   = globalPrefix + "doc:"         // doc:{id}
	KeyDocContentPrefix = globalPrefix + "doc:content:" // doc:content:{id}
	KeyDocMetaPrefix    = globalPrefix + "doc:meta:"    // doc:meta:{id}

	// Space cache keys
	KeySpacePrefix = globalPrefix + "space:" // space:{id}
	KeySpaceList   = globalPrefix + "space:list"

	// Category cache keys
	KeyCategoryTree   = globalPrefix + "cat:tree:" // cat:tree:{spaceId}
	KeyCategoryPrefix = globalPrefix + "cat:"      // cat:{id}

	// Tag cache keys
	KeyTagList   = globalPrefix + "tags:list"
	KeyTagPrefix = globalPrefix + "tags:" // tags:{id}

	// Search cache keys
	KeySearchPrefix = globalPrefix + "search:" // search:{queryHash}

	// Session cache keys
	KeySessionPrefix = globalPrefix + "session:" // session:{token}

	// Rate limiting cache keys
	KeyRateLimitPrefix = globalPrefix + "ratelimit:" // ratelimit:{ip}:{action}

	// Admin/system cache keys
	KeySystemConfig = globalPrefix + "sys:config"
	KeySystemStats  = globalPrefix + "sys:stats"
)

// UserKey generates a cache key for a user by ID.
func UserKey(id uint) string {
	return fmt.Sprintf("%s%d", KeyUserPrefix, id)
}

// UserByNameKey generates a cache key for a user by username.
func UserByNameKey(name string) string {
	return KeyUserByName + name
}

// UserTokenKey generates a cache key for a user token.
func UserTokenKey(token string) string {
	return KeyUserToken + token
}

// DocumentKey generates a cache key for a document by ID.
func DocumentKey(id uint) string {
	return fmt.Sprintf("%s%d", KeyDocumentPrefix, id)
}

// DocContentKey generates a cache key for document content by ID.
func DocContentKey(id uint) string {
	return fmt.Sprintf("%s%d", KeyDocContentPrefix, id)
}

// DocMetaKey generates a cache key for document metadata by ID.
func DocMetaKey(id uint) string {
	return fmt.Sprintf("%s%d", KeyDocMetaPrefix, id)
}

// SpaceKey generates a cache key for a space by ID.
func SpaceKey(id uint) string {
	return fmt.Sprintf("%s%d", KeySpacePrefix, id)
}

// CategoryTreeKey generates a cache key for a space's category tree.
func CategoryTreeKey(spaceID uint) string {
	return fmt.Sprintf("%s%d", KeyCategoryTree, spaceID)
}

// CategoryKey generates a cache key for a category by ID.
func CategoryKey(id uint) string {
	return fmt.Sprintf("%s%d", KeyCategoryPrefix, id)
}

// TagKey generates a cache key for a tag by ID.
func TagKey(id uint) string {
	return fmt.Sprintf("%s%d", KeyTagPrefix, id)
}

// SearchKey generates a cache key for a search query.
func SearchKey(queryHash string) string {
	return KeySearchPrefix + queryHash
}

// SessionKey generates a cache key for a session token.
func SessionKey(token string) string {
	return KeySessionPrefix + token
}

// RateLimitKey generates a cache key for rate limiting.
func RateLimitKey(ip, action string) string {
	return fmt.Sprintf("%s%s:%s", KeyRateLimitPrefix, ip, action)
}
