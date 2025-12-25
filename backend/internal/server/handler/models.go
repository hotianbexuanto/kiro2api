package handler

import (
	"net/http"

	"kiro2api/internal/config"
	"kiro2api/internal/types"

	"github.com/gin-gonic/gin"
)

// HandleModels GET /v1/models - 返回可用模型列表
func HandleModels(c *gin.Context) {
	models := []types.Model{}
	for anthropicModel := range config.ModelMap {
		model := types.Model{
			ID:          anthropicModel,
			Object:      "model",
			Created:     1234567890,
			OwnedBy:     "anthropic",
			DisplayName: anthropicModel,
			Type:        "text",
			MaxTokens:   200000,
		}
		models = append(models, model)
	}

	response := types.ModelsResponse{
		Object: "list",
		Data:   models,
	}

	c.JSON(http.StatusOK, response)
}
