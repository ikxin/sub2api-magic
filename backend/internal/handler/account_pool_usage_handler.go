package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type AccountPoolUsageHandler struct {
	accountUsage *service.AccountUsageService
	concurrency  *service.ConcurrencyService
}

func NewAccountPoolUsageHandler(accountUsage *service.AccountUsageService, concurrency *service.ConcurrencyService) *AccountPoolUsageHandler {
	return &AccountPoolUsageHandler{accountUsage: accountUsage, concurrency: concurrency}
}

// List handles GET /api/v1/usage/account-pool.
func (h *AccountPoolUsageHandler) List(c *gin.Context) {
	if _, ok := middleware.GetAuthSubjectFromContext(c); !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	ctx := c.Request.Context()
	items, err := h.accountUsage.ListPoolUsage(ctx)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	ids := make([]int64, len(items))
	for i := range items {
		ids[i] = items[i].ID
	}
	if counts, err := h.concurrency.ReadAccountConcurrencyBatch(ctx, ids); err == nil {
		for i := range items {
			if count, ok := counts[items[i].ID]; ok {
				items[i].CurrentConcurrency = &count
			}
		}
	}

	c.Header("Cache-Control", "private, no-store")
	response.Success(c, items)
}
