package admin

import (
	"crypto/tls"
	"database/sql"
	"errors"

	"github.com/listen808/citebase/internal/brand"
	adminmodel "github.com/listen808/citebase/internal/model/admin"
	adminrepo "github.com/listen808/citebase/internal/repository/admin"
	"github.com/listen808/citebase/internal/utils"
	"gorm.io/gorm"

	"fmt"
	"strings"

	"github.com/go-mail/mail/v2"
)

type EmailService struct {
	emailRepo *adminrepo.EmailRepository
}

func NewEmailService() *EmailService {
	return &EmailService{
		emailRepo: adminrepo.NewEmailRepository(),
	}
}

var (
	ErrEmailNameExists = errors.New("email server name already exists")
	ErrEmailNotFound   = errors.New("email server not found")
	ErrEmailTestFailed = errors.New("email test failed")
)

func (s *EmailService) CreateEmail(req *adminmodel.CreateEmailRequest) (*adminmodel.Email, error) {
	exists, err := s.emailRepo.HasName(req.Name)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrEmailNameExists
	}
	// SMTP 口令入库前加密，与数据源同一套（见 utils/crypto.go）。
	// 理由一样：aw_email 的这列此前是明文，而整库导出（make db-dump / 系统备份）
	// 会把邮件凭据一起写进 SQL 文件里。
	sealedPassword, err := utils.EncryptSecret(req.Password)
	if err != nil {
		return nil, fmt.Errorf("加密 SMTP 口令失败: %w", err)
	}

	email := &adminmodel.Email{
		Name:              req.Name,
		SenderAddress:     req.SenderAddress,
		SenderName:        req.SenderName,
		SenderTitlePrefix: req.SenderTitlePrefix,
		Host:              req.Host,
		Port:              req.Port,
		Username:          req.Username,
		Password:          sealedPassword,
		IsSSL:             req.IsSSL,
	}

	count, _ := s.emailRepo.Count()
	if count == 0 {
		email.IsUsed = 1
	}

	if err := s.emailRepo.Create(email); err != nil {
		return nil, err
	}
	return email, nil
}

func (s *EmailService) GetEmailByID(id int) (*adminmodel.EmailResponse, error) {
	email, err := s.emailRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, sql.ErrNoRows) {
			return nil, ErrEmailNotFound
		}
		return nil, err
	}
	resp := email.ToResponse()
	return &resp, nil
}

func (s *EmailService) GetAllEmails() ([]*adminmodel.EmailResponse, error) {
	emails, err := s.emailRepo.FindAll()
	if err != nil {
		return nil, err
	}
	responses := make([]*adminmodel.EmailResponse, len(emails))
	for i, e := range emails {
		resp := e.ToResponse()
		responses[i] = &resp
	}
	return responses, nil
}

func (s *EmailService) GetEmailsByPage(page, pageSize int, keyword string) ([]*adminmodel.EmailResponse, int64, error) {
	offset := (page - 1) * pageSize
	emails, total, err := s.emailRepo.FindByKeyword(keyword, offset, pageSize)
	if err != nil {
		return nil, 0, err
	}
	responses := make([]*adminmodel.EmailResponse, len(emails))
	for i, e := range emails {
		resp := e.ToResponse()
		responses[i] = &resp
	}
	return responses, total, nil
}

func (s *EmailService) UpdateEmail(id int, req *adminmodel.UpdateEmailRequest) (*adminmodel.EmailResponse, error) {
	email, err := s.emailRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, sql.ErrNoRows) {
			return nil, ErrEmailNotFound
		}
		return nil, err
	}

	conflict, err := s.emailRepo.HasNameExcludeID(id, req.Name)
	if err != nil {
		return nil, err
	}
	if conflict {
		return nil, ErrEmailNameExists
	}

	email.Name = req.Name
	email.SenderAddress = req.SenderAddress
	email.SenderName = req.SenderName
	email.SenderTitlePrefix = req.SenderTitlePrefix
	email.Host = req.Host
	email.Port = req.Port
	email.Username = req.Username
	email.IsSSL = req.IsSSL
	if req.Password != "" {
		// 空串 = 没改口令（保留库里已有值，上面那条 if 已经是对的）；
		// 已经是密文则不重复加密，否则密文上再套一层就解不回明文了。
		if utils.LooksEncrypted(req.Password) {
			return nil, errors.New("SMTP 口令字段收到的是密文，请提交明文或在表单里重新输入")
		}
		sealed, err := utils.EncryptSecret(req.Password)
		if err != nil {
			return nil, fmt.Errorf("加密 SMTP 口令失败: %w", err)
		}
		email.Password = sealed
	}

	if err := s.emailRepo.Update(email); err != nil {
		return nil, err
	}
	resp := email.ToResponse()
	return &resp, nil
}

func (s *EmailService) DeleteEmail(id int) error {
	email, err := s.emailRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, sql.ErrNoRows) {
			return ErrEmailNotFound
		}
		return err
	}
	return s.emailRepo.Delete(email.ID)
}

func (s *EmailService) SetDefaultEmail(id int) error {
	email, err := s.emailRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, sql.ErrNoRows) {
			return ErrEmailNotFound
		}
		return err
	}

	if err := s.emailRepo.UnsetAllUsed(); err != nil {
		return err
	}
	return s.emailRepo.SetUsed(email.ID)
}

func (s *EmailService) TestEmail(req *adminmodel.TestEmailRequest) error {
	m := mail.NewMessage()

	m.SetAddressHeader("From", req.SenderAddress, req.SenderName)

	toEmails := strings.Split(req.ToEmails, ";")
	addresses := make([]string, 0, len(toEmails))
	for _, e := range toEmails {
		e = strings.TrimSpace(e)
		if e != "" {
			addresses = append(addresses, e)
		}
	}
	if len(addresses) == 0 {
		return errors.New("no valid recipient email addresses")
	}
	// SetHeader 是覆盖语义（每次赋值 m.header["To"]），循环单地址调用只会留下最后一个收件人。
	// 必须一次传入全部地址，前面的收件人才不会静默收不到信。
	m.SetHeader("To", addresses...)

	subject := req.SenderTitlePrefix
	if subject != "" {
		subject = subject + " - "
	}
	siteName := brand.SiteName()
	subject += siteName + " 测试邮件"
	m.SetHeader("Subject", subject)

	// 站点名走实参而不是拼进格式串：管理员给站点起的名字里出现一个 `%` 就会让
	// Sprintf 少读一个参数，产出一封写着 `%!s(MISSING)` 的信。索引式引用同理。
	body := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head><meta charset="utf-8"></head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif; color: #1d1d1f; padding: 40px 20px;">
  <div style="max-width: 600px; margin: 0 auto;">
    <h2 style="color: #1d1d1f; font-size: 24px; font-weight: 600;">%[1]s 测试邮件</h2>
    <p style="color: #86868b; font-size: 15px; line-height: 1.6;">您好，</p>
    <p style="color: #1d1d1f; font-size: 15px; line-height: 1.6;">
      这是一封来自 <strong>%[1]s</strong> 的测试邮件，用于验证邮件服务器配置是否正确。
    </p>
    <div style="background: #f5f5f7; border-radius: 12px; padding: 20px; margin: 24px 0;">
      <p style="margin: 0; color: #86868b; font-size: 13px;">
        <strong>服务器信息：</strong><br>
        主机：%[2]s<br>
        端口：%[3]s<br>
        用户名：%[4]s<br>
        SSL：%[5]s
      </p>
    </div>
    <p style="color: #86868b; font-size: 13px;">
      请勿回复此邮件。如有问题，请联系管理员。
    </p>
  </div>
</body>
</html>`, siteName, req.Host, req.Port, req.Username, map[bool]string{true: "启用", false: "未启用"}[req.IsSSL == 1])

	m.SetBody("text/html", body)

	port := 587
	if req.IsSSL == 1 {
		port = 465
	}

	if err := sendEmailWithDialer(m, req.Host, port, req.Username, req.Password, req.IsSSL == 1); err != nil {
		return fmt.Errorf("%w: %v", ErrEmailTestFailed, err)
	}
	return nil
}

// SendRaw 发送一封原始邮件（用于主动投递等通用场景）。
// subject 为邮件主题，htmlBody 为 HTML 正文，toEmails 为收件地址列表。
// 使用系统中标记为默认的邮件服务器配置。
func (s *EmailService) SendRaw(subject, htmlBody string, toEmails []string) error {
	usedEmail, err := s.emailRepo.GetUsedEmail()
	if err != nil {
		return fmt.Errorf("no email server configured: %w", err)
	}
	if len(toEmails) == 0 {
		return fmt.Errorf("no recipients")
	}
	m := mail.NewMessage()
	m.SetAddressHeader("From", usedEmail.SenderAddress, usedEmail.SenderName)
	m.SetHeader("Subject", subject)
	// 一次性设置全部收件人：SetHeader 覆盖语义下循环逐个传只会保留最后一个
	m.SetHeader("To", toEmails...)
	m.SetBody("text/html", htmlBody)
	port := 587
	if usedEmail.IsSSL == 1 {
		port = 465
	}
	return sendEmailWithDialer(m, usedEmail.Host, port, usedEmail.Username, utils.DecryptSecretOrPlain(usedEmail.Password), usedEmail.IsSSL == 1)
}

func sendEmailWithDialer(m *mail.Message, host string, port int, username, password string, isSSL bool) error {
	d := mail.NewDialer(host, port, username, password)
	if isSSL {
		d.SSL = true
		d.TLSConfig = &tls.Config{InsecureSkipVerify: false, MinVersion: tls.VersionTLS12}
	}
	return d.DialAndSend(m)
}

func (s *EmailService) SendDocumentNotification(docName, editorName, changeLog, docURL string, toEmails []string) error {
	usedEmail, err := s.emailRepo.GetUsedEmail()
	if err != nil {
		return fmt.Errorf("no email server configured: %w", err)
	}

	m := mail.NewMessage()
	m.SetAddressHeader("From", usedEmail.SenderAddress, usedEmail.SenderName)

	subject := usedEmail.SenderTitlePrefix
	if subject != "" {
		subject += " - "
	}
	subject += fmt.Sprintf("「%s」文档已更新", docName)
	m.SetHeader("Subject", subject)

	// 关注者列表往往有多个地址：SetHeader 为覆盖语义，必须一次传入，
	// 否则除最后一人之外的所有关注者都收不到文档更新通知。
	m.SetHeader("To", toEmails...)

	body := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head><meta charset="utf-8"></head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif; color: #1d1d1f; padding: 40px 20px;">
  <div style="max-width: 600px; margin: 0 auto;">
    <h2 style="color: #1d1d1f; font-size: 24px; font-weight: 600;">📄 文档更新通知</h2>
    <p style="color: #86868b; font-size: 15px;">%s 修改了文档：</p>
    <div style="background: #f5f5f7; border-radius: 12px; padding: 20px; margin: 24px 0;">
      <p style="margin: 0 0 12px 0; font-size: 18px; font-weight: 600; color: #1d1d1f;">%s</p>
      <p style="margin: 0 0 8px 0; color: #86868b; font-size: 14px;">更新说明：%s</p>
      <p style="margin: 0; color: #86868b; font-size: 14px;">更新时间：%s</p>
    </div>
    <a href="%s" style="display: inline-block; background: #0071e3; color: #fff; padding: 12px 28px; border-radius: 980px; text-decoration: none; font-size: 15px; font-weight: 500;">
      查看文档 →
    </a>
    <p style="color: #86868b; font-size: 13px; margin-top: 32px;">
      您收到此邮件是因为您关注了该文档的更新。如不想再收到通知，请在文档页面取消关注。
    </p>
  </div>
</body>
</html>`,
		editorName, docName, changeLog,
		"刚刚",
		docURL)

	m.SetBody("text/html", body)

	port := 587
	if usedEmail.IsSSL == 1 {
		port = 465
	}
	return sendEmailWithDialer(m, usedEmail.Host, port, usedEmail.Username, utils.DecryptSecretOrPlain(usedEmail.Password), usedEmail.IsSSL == 1)
}
