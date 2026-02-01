package conversation_msg

import (
	"context"

	"github.com/openimsdk/openim-sdk-core/v3/pkg/db/model_struct"
)

func (c *Conversation) getConversationWithCache(ctx context.Context, conversationID string) (*model_struct.LocalConversation, error) {
	if c.cache != nil {
		if cached, ok := c.cache.Load(conversationID); ok && cached != nil {
			return cached, nil
		}
	}
	lc, err := c.db.GetConversation(ctx, conversationID)
	if err != nil {
		return nil, err
	}
	if c.cache != nil {
		c.cache.Store(conversationID, lc)
	}
	return lc, nil
}

func (c *Conversation) getMultipleConversationWithCache(ctx context.Context, conversationIDs []string) ([]*model_struct.LocalConversation, error) {
	if len(conversationIDs) == 0 {
		return nil, nil
	}
	if c.cache == nil {
		return c.db.GetMultipleConversationDB(ctx, conversationIDs)
	}

	cached := make(map[string]*model_struct.LocalConversation, len(conversationIDs))
	missingSet := make(map[string]struct{}, len(conversationIDs))
	for _, conversationID := range conversationIDs {
		if conversationID == "" {
			continue
		}
		if lc, ok := c.cache.Load(conversationID); ok && lc != nil {
			cached[conversationID] = lc
			continue
		}
		missingSet[conversationID] = struct{}{}
	}

	if len(missingSet) > 0 {
		missing := make([]string, 0, len(missingSet))
		for conversationID := range missingSet {
			missing = append(missing, conversationID)
		}
		dbList, err := c.db.GetMultipleConversationDB(ctx, missing)
		if err != nil {
			return nil, err
		}
		for _, lc := range dbList {
			if lc == nil {
				continue
			}
			c.cache.Store(lc.ConversationID, lc)
			cached[lc.ConversationID] = lc
		}
	}

	result := make([]*model_struct.LocalConversation, 0, len(cached))
	seen := make(map[string]struct{}, len(cached))
	for _, conversationID := range conversationIDs {
		if conversationID == "" {
			continue
		}
		if _, ok := seen[conversationID]; ok {
			continue
		}
		if lc, ok := cached[conversationID]; ok && lc != nil {
			result = append(result, lc)
		}
		seen[conversationID] = struct{}{}
	}
	return result, nil
}
