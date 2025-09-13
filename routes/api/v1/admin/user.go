package admin

import (
	"PJS_Exchange/app"
	"PJS_Exchange/databases/postgres"
	"PJS_Exchange/middleware"
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
)

type UserRouter struct{}

func (ur *UserRouter) RegisterRoutes(router fiber.Router) {
	adminUserGroup := router.Group("/user", middleware.AuthAPIKeyMiddlewareRequireScopes(postgres.APIKeyScope{
		AdminUserManage: true,
	}))

	// 유저 목록 조회
	adminUserGroup.Get("/", ur.getUserList)
	adminUserGroup.Get("/:id", ur.getUserDetail)
	adminUserGroup.Post("/", ur.generateAccessCode)
	adminUserGroup.Patch("/:id/activate", ur.activateUser)     // 유저 활성화
	adminUserGroup.Patch("/:id/deactivate", ur.deactivateUser) // 유저 비
}

// === 핸들러 함수들 ===

// @Summary		등록된 모든 유저 목록 반환
// @Description	모든 유저의 [ID, 이름, 활성화 여부] 목록을 배열로 반환합니다.
// @Tags			Admin - User
// @Produce		json
// @Param			Authorization	header		string				true	"Bearer {API_KEY}"	with	AdminUserManage	Scope
// @Success		200				{object}	map[string][]int	"성공 시 유저 목록 반환"
// @Failure		500				{object}	map[string]string	"서버 오류 시 에러 메시지 반환"
// @Failure		401				{object}	map[string]string	"인증 실패 시 에러 메시지 반환"
// @Router			/api/v1/admin/user [get]
func (ur *UserRouter) getUserList(c *fiber.Ctx) error {
	ctx := context.Background()
	userList, err := app.GetApp().UserRepo().GetUserIDs(ctx)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to get user list: " + err.Error(),
			"code":  fiber.StatusInternalServerError,
		})
	}
	return c.Status(200).JSON(fiber.Map{
		"user_ids": userList,
	})
}

// @Summary		특정 유저의 상세 정보 반환
// @Description	특정 유저의 상세 정보를 반환합니다.
// @Tags			Admin - User
// @Produce		json
// @Param			id				path		int					true	"유저 ID"
// @Param			Authorization	header		string				true	"Bearer {API_KEY}"	with	AdminUserManage	Scope
// @Success		200				{object}	postgres.User		"성공 시 유저 상세 정보 반환"
// @Failure		400				{object}	map[string]string	"잘못된 요청 시 에러 메시지 반환"
// @Failure		404				{object}	map[string]string	"유저를 찾을 수 없을 때 에러 메시지 반환"
// @Failure		500				{object}	map[string]string	"서버 오류 시 에러 메시지 반환"
// @Failure		401				{object}	map[string]string	"인증 실패 시 에러 메시지 반환"
// @Router			/api/v1/admin/user/{id} [get]
func (ur *UserRouter) getUserDetail(c *fiber.Ctx) error {
	ctx := context.Background()
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid user ID",
			"code":  fiber.StatusBadRequest,
		})
	}
	user, err := app.GetApp().UserRepo().GetUserByID(ctx, id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get user detail: " + err.Error(),
			"code":  fiber.StatusInternalServerError,
		})
	}
	if user == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "User not found",
			"code":  fiber.StatusNotFound,
		})
	}
	return c.Status(fiber.StatusOK).JSON(user)
}

// @Summary		새로운 유저 등록용 액세스 코드 생성
// @Description	새로운 유저 등록에 사용할 수 있는 액세스 코드를 생성합니다.
// @Tags			Admin - User
// @Produce		json
// @Param			Authorization	header		string				true	"Bearer {API_KEY}"	with	AdminUserManage	Scope
// @Success		201				{object}	map[string]string	"성공 시 액세스 코드 반환"
// @Failure		500				{object}	map[string]string	"서버 오류 시 에러 메시지 반환"
// @Failure		401				{object}	map[string]string	"인증 실패 시 에러 메시지 반환"
// @Router			/api/v1/admin/user [post]
func (ur *UserRouter) generateAccessCode(c *fiber.Ctx) error {
	ctx := context.Background()
	TimeA := time.Now().Add(24 * time.Hour)
	accessCode, data, err := app.GetApp().AcceptCodeRepo().GenerateAcceptCode(ctx, &TimeA)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to generate access code: " + err.Error(),
			"code":  fiber.StatusInternalServerError,
		})
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"access_code": accessCode,
		"data":        data,
	})
}

// @Summary		유저 활성화
// @Description	특정 유저를 활성화합니다. 활성화된 유저는 로그인 및 API 접근이 가능합니다.
// @Tags			Admin - User
// @Produce		json
// @Param			id				path		int					true	"유저 ID"
// @Param			Authorization	header		string				true	"Bearer {API_KEY}"	with	AdminUserManage	Scope
// @Success		200				{object}	map[string]string	"성공 시 성공 메시지 반환"
// @Failure		404				{object}	map[string]string	"유저를 찾을 수 없을 때 에러 메시지 반환"
// @Failure		500				{object}	map[string]string	"서버 오류 시 에러 메시지 반환"
// @Failure		401				{object}	map[string]string	"인증 실패 시 에러 메시지 반환"
// @Router			/api/v1/admin/user/{id}/activate [patch]
func (ur *UserRouter) activateUser(c *fiber.Ctx) error {
	ctx := context.Background()
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Invalid user ID",
			"code":  fiber.StatusNotFound,
		})
	}
	err = app.GetApp().UserRepo().EnableUser(ctx, id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to activate user: " + err.Error(),
			"code":  fiber.StatusInternalServerError,
		})
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "User activated successfully",
	})
}

// @Summary		유저 비활성화
// @Description	특정 유저를 비활성화합니다. 비활성화된 유저는 로그인 및 API 접근이 불가능합니다.
// @Tags			Admin - User
// @Produce		json
// @Param			id				path		int					true	"유저 ID"
// @Param			Authorization	header		string				true	"Bearer {API_KEY}"	with	AdminUserManage	Scope
// @Success		200				{object}	map[string]string	"성공 시 성공 메시지 반환"
// @Failure		404				{object}	map[string]string	"유저를 찾을 수 없을 때 에러 메시지 반환"
// @Failure		500				{object}	map[string]string	"서버 오류 시 에러 메시지 반환"
// @Failure		401				{object}	map[string]string	"인증 실패 시 에러 메시지 반환"
// @Router			/api/v1/admin/user/{id}/deactivate [patch]
func (ur *UserRouter) deactivateUser(c *fiber.Ctx) error {
	ctx := context.Background()
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Invalid user ID",
			"code":  fiber.StatusNotFound,
		})
	}
	err = app.GetApp().UserRepo().DisableUser(ctx, id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to deactivate user: " + err.Error(),
			"code":  fiber.StatusInternalServerError,
		})
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "User deactivated successfully",
	})
}
