package controllers

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"fluxguard/server/web/views/login/models"
)

func TestCaptchaRoundTrip(t *testing.T) {
	captcha := NewCaptcha()
	resp, err := captcha.Generate(context.Background(), &models.CaptchaRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.CaptchaId == "" || !strings.HasPrefix(resp.Image, "data:image/png;base64,") || resp.ExpireAt == 0 {
		t.Fatalf("captcha response %+v", resp)
	}
	if strings.Contains(resp.Image, "000000") {
		t.Fatal("image must not carry the answer as text")
	}
	if err := captcha.Verify(resp.CaptchaId, "000000"); !errors.Is(err, ErrCaptchaInvalid) && !errors.Is(err, ErrCaptchaExpired) {
		t.Fatalf("wrong code err=%v", err)
	}
	if err := captcha.Verify(resp.CaptchaId, "000000"); !errors.Is(err, ErrCaptchaExpired) {
		t.Fatalf("reused ticket err=%v", err)
	}
}

func TestCaptchaTicketCannotBeReused(t *testing.T) {
	captcha := NewCaptcha()
	ticket, err := captcha.signTicket("135790", time.Now().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if err := captcha.Verify(ticket, "135790"); err != nil {
		t.Fatal(err)
	}
	if err := captcha.Verify(ticket, "135790"); !errors.Is(err, ErrCaptchaExpired) {
		t.Fatalf("reused ticket err=%v", err)
	}
	wrong, err := captcha.signTicket("135790", time.Now().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if err := captcha.Verify(wrong, "000000"); !errors.Is(err, ErrCaptchaInvalid) {
		t.Fatalf("wrong code err=%v", err)
	}
	if err := captcha.Verify(wrong, "135790"); !errors.Is(err, ErrCaptchaExpired) {
		t.Fatalf("wrong answer should still consume the ticket, err=%v", err)
	}
}
