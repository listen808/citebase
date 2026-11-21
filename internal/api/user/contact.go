package user

import (
	"strconv"

	"github.com/gin-gonic/gin"
	usermodel "github.com/listen808/citebase/internal/model/user"
	userrepo "github.com/listen808/citebase/internal/repository/user"
	"github.com/listen808/citebase/internal/response"
	usersvc "github.com/listen808/citebase/internal/service/user"
	"github.com/listen808/citebase/internal/utils"
)

// ContactAPI 联系人 API
type ContactAPI struct {
	contactRepo *userrepo.ContactRepository
	userService *usersvc.UserService
}

func NewContactAPI() *ContactAPI {
	return &ContactAPI{
		contactRepo: userrepo.NewContactRepository(),
		userService: usersvc.NewUserService(),
	}
}

// GetContacts 获取联系人列表
func (api *ContactAPI) GetContacts(c *gin.Context) {
	page, pageSize, _ := utils.ParsePagination(c, 20)
	keyword := c.DefaultQuery("keyword", "")

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	contacts, total, err := api.contactRepo.List(page, pageSize, keyword)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.Paginated(c, contacts, total, page, pageSize)
}

// GetAllContacts 获取全部联系人
func (api *ContactAPI) GetAllContacts(c *gin.Context) {
	contacts, err := api.contactRepo.ListAll()
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	response.Success(c, contacts)
}

// SaveContact 新增联系人
func (api *ContactAPI) SaveContact(c *gin.Context) {
	var req struct {
		Name     string `json:"name" binding:"required"`
		Mobile   string `json:"mobile"`
		Email    string `json:"email" binding:"required"`
		Position string `json:"position"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequestKey(c, response.MsgContactFieldsEmpty)
		return
	}

	contact := &usermodel.Contact{
		Name:     req.Name,
		Mobile:   req.Mobile,
		Email:    req.Email,
		Position: req.Position,
	}
	if err := api.contactRepo.Create(contact); err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.SuccessWithMsg(c, contact, "添加成功")
}

// UpdateContact 修改联系人
func (api *ContactAPI) UpdateContact(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if id == 0 {
		response.BadRequestKey(c, response.MsgParameterError)
		return
	}

	contact, err := api.contactRepo.FindByID(uint(id))
	if err != nil {
		response.NotFoundKey(c, response.MsgContactNotFound)
		return
	}

	var req struct {
		Name     string `json:"name" binding:"required"`
		Mobile   string `json:"mobile"`
		Email    string `json:"email" binding:"required"`
		Position string `json:"position"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequestKey(c, response.MsgContactFieldsEmpty)
		return
	}

	contact.Name = req.Name
	contact.Mobile = req.Mobile
	contact.Email = req.Email
	contact.Position = req.Position

	if err := api.contactRepo.Update(contact); err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.SuccessWithMsg(c, contact, "修改成功")
}

// DeleteContact 删除联系人
func (api *ContactAPI) DeleteContact(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if id == 0 {
		response.BadRequestKey(c, response.MsgNoContactChosen)
		return
	}

	_, err := api.contactRepo.FindByID(uint(id))
	if err != nil {
		response.NotFoundKey(c, response.MsgContactNotFound)
		return
	}

	if err := api.contactRepo.Delete(uint(id)); err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.SuccessWithMsg(c, nil, "删除成功")
}

// GetContact 获取单个联系人
func (api *ContactAPI) GetContact(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	contact, err := api.contactRepo.FindByID(uint(id))
	if err != nil {
		response.NotFoundKey(c, response.MsgContactNotFound)
		return
	}
	response.Success(c, contact)
}

// ImportContactsFromUsers 从系统用户导入联系人
func (api *ContactAPI) ImportContactsFromUsers(c *gin.Context) {
	users, err := api.userService.GetAllUsersBasic()
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	imported := 0
	skipped := 0
	for _, user := range users {
		// 只导入有邮箱的用户
		email, ok := user["email"].(string)
		if !ok || email == "" {
			skipped++
			continue
		}

		// P3：username 类型断言加 ok 检查，避免 nil panic
		username, _ := user["username"].(string)
		contact := &usermodel.Contact{
			Name:     getString(user, "given_name", username),
			Mobile:   getString(user, "mobile", ""),
			Email:    email,
			Position: getString(user, "position", ""),
		}

		if err := api.contactRepo.Create(contact); err != nil {
			skipped++
			continue
		}
		imported++
	}

	response.SuccessWithMsg(c, gin.H{
		"imported": imported,
		"skipped":  skipped,
	}, "导入完成")
}

func getString(m map[string]interface{}, key, fallback string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return fallback
}
