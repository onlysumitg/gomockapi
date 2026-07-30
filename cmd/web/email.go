package main

import (
	"github.com/onlysumitg/GoMockAPI/internal/models"
	mail "github.com/xhit/go-simple-mail/v2"
)

// ------------------------------------------------------
//
// ------------------------------------------------------
// func (app *application) SampleEmail() {
// 	e := &models.EmailRequest{
// 		To:       []string{"onlysumitg@gmail.com"},
// 		Subject:  "Test email 32",
// 		Body:     " this is test email2",
// 		Template: "email_verify_email.tmpl",
// 	}

// 	app.SendEmail(e)
// }

// ------------------------------------------------------
//
// ------------------------------------------------------
// SendEmail delivers r, returning any failure to the caller.
//
// It must never terminate the process: it runs on background goroutines spawned
// by request handlers, so exiting here would take the whole mock server down
// whenever SMTP is unreachable or unconfigured.
func (app *application) SendEmail(r *models.EmailRequest) error {

	if r == nil {
		return nil
	}

	smtpClient, err := app.EmailServer.Connect()
	if err != nil {
		app.errorLog.Printf("email: connecting to %s:%d failed: %v", app.EmailServer.Host, app.EmailServer.Port, err)
		return err
	}

	// Create email
	email := mail.NewMSG()
	email.SetFrom("gomockapi@zerobit.tech")
	email.AddTo(r.To...)
	//email.AddCc("another_you@example.com")
	email.SetSubject(r.Subject)

	if r.Template != "" {
		tBody, err := app.templateToString(r.Template, r.Data)

		if err == nil && tBody != "" {
			r.Body = tBody
		}
	}

	email.SetBody(mail.TextHTML, r.Body)

	//email.AddAttachment("super_cool_file.png")

	// Send email
	err = email.Send(smtpClient)
	if err != nil {
		app.errorLog.Printf("email: sending %q to %v failed: %v", r.Subject, r.To, err)
		return err
	}

	return nil
}
