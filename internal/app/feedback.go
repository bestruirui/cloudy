package app

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// Feedback goes through FormSubmit, which mails what is posted to it: an app
// can't hold the password of a mailbox, for anyone could read it out of it.
// Not to its /ajax/ address, which drops attachments.
var feedbackTo = "https://formsubmit.co/nya84855@gmail.com"

// feedbackFrom is what names the app to FormSubmit, which goes by where a
// form is.
const feedbackFrom = "https://mygo-cloudmusic.nya.invalid/feedback"

// How much a message may hold: FormSubmit takes 10 MB in all. It can take
// most of a minute to answer, so a message goes in the background, and is given
// time.
const (
	feedbackImages = 6
	feedbackBytes  = 10 << 20
	feedbackWait   = 3 * time.Minute
	feedbackEvery  = time.Minute // apart, for a mailbox not to fill up
)

// errFeedbackInactive says that the mailbox hasn't confirmed FormSubmit yet:
// nothing is sent until it has, from the mail FormSubmit sends it.
var errFeedbackInactive = errors.New("feedback: the form needs activation")

// A feedbackImage is a picture to send: where it is, and what shows of it.
type feedbackImage struct {
	path string
	size int64
	bmp  *ui.Bitmap
}

// feedbackState is the message being written.
type feedbackState struct {
	text    string
	images  []feedbackImage
	sending bool
	sentAt  time.Time
	note    string
	failed  bool
}

// feedbackCard is the message, what it is sent with, and the button to send it.
func (a *app) feedbackCard(c *ui.Context, ink paneInk) {
	f := &a.feedback
	ui.Column(c).Padding(14, 16).Gap(12).AlignItems(ui.Stretch).Children(func() {
		ui.Text(c, feedbackIntro()).FontSize(12).TextColor(ink.muted).LineHeight(1.4)
		ui.TextArea(c.Key("feedback-text"), &f.text).Placeholder("写下你想说的…").Label("反馈内容").Height(110).FontSize(13.5)
		if len(f.images) > 0 {
			gone := -1
			ui.Row(c).Gap(8).Children(func() {
				for i := range f.images {
					im := &f.images[i]
					ui.Box(c.Key(im.path)).Size(64, 64).Radius(8).Children(func() {
						artImage(c, im.bmp, 8, 1)
						x := iconButton(c.Key("remove"), iconX, 11, white, "移除图片").Absolute().Top(3).Right(3).Size(20, 20).Background(ui.RGBA(0, 0, 0, 0.55))
						if x.Clicked() {
							gone = i
						}
					})
				}
			})
			if gone >= 0 {
				f.images = append(f.images[:gone:gone], f.images[gone+1:]...)
			}
		}
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			add := ui.Button(c.Key("feedback-add"), "添加图片").Disabled(f.sending || len(f.images) >= feedbackImages)
			if add.Clicked() {
				a.pickFeedbackImages()
			}
			note := ink.muted
			if f.failed {
				note = a.pal.accent
			}
			ui.Text(c, f.note).FontSize(12).TextColor(note).Grow(1).Shrink(1).MinWidth(0).MaxLines(2).LineHeight(1.35)
			label := "发送"
			if f.sending {
				label = "后台发送中…"
			}
			empty := strings.TrimSpace(f.text) == ""
			if feedbackBilibiliUID == 0 {
				if ui.PrimaryButton(c.Key("feedback-send"), label).Disabled(f.sending || empty).Clicked() {
					a.sendFeedback()
				}
				return
			}
			// Mail goes through a form some networks cannot reach: the
			// author's Bilibili messages come first.
			if ui.Button(c.Key("feedback-send"), "用邮件发送").Disabled(f.sending || empty).Clicked() {
				a.sendFeedback()
			}
			if ui.PrimaryButton(c.Key("feedback-bilibili"), "发到 B 站私信").Disabled(empty).Clicked() {
				a.feedbackToBilibili()
			}
		})
	})
}

// pickFeedbackImages asks for pictures to send: the dialog waits for the
// main thread, which is the one showing it.
func (a *app) pickFeedbackImages() {
	win := a.win
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: win, Title: "选择图片", Multiple: true,
			Filters: []mygo.FileFilter{{Name: "图片", Extensions: []string{"png", "jpg", "jpeg", "webp"}}}})
		if err != nil || len(paths) == 0 {
			return
		}
		var loaded []feedbackImage
		for _, path := range paths {
			if im, err := loadFeedbackImage(path); err == nil {
				loaded = append(loaded, im)
			}
		}
		broken := len(loaded) < len(paths)
		a.post(func() { a.addFeedbackImages(loaded, broken) })
	}()
}

func loadFeedbackImage(path string) (feedbackImage, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return feedbackImage{}, err
	}
	img, _, err := image.Decode(bytes.NewReader(b))
	if err != nil {
		return feedbackImage{}, err
	}
	return feedbackImage{path: path, size: int64(len(b)), bmp: ui.NewBitmap(img)}, nil
}

// addFeedbackImages adds what fits of loaded, and says what didn't.
func (a *app) addFeedbackImages(loaded []feedbackImage, broken bool) {
	f := &a.feedback
	total := int64(0)
	for _, im := range f.images {
		total += im.size
	}
	f.note, f.failed = "", false
	if broken {
		f.note, f.failed = "有图片无法读取", true
	}
	for _, im := range loaded {
		switch {
		case len(f.images) >= feedbackImages:
			f.note, f.failed = fmt.Sprintf("最多添加 %d 张图片", feedbackImages), true
			return
		case total+im.size > feedbackBytes:
			f.note, f.failed = "图片总大小不能超过 10MB", true
			return
		}
		f.images, total = append(f.images, im), total+im.size
	}
}

// sendFeedback sends the message in the background, for the listener not to
// wait on it, and says how it went. If it can't go, it comes back as written.
func (a *app) sendFeedback() {
	f := &a.feedback
	text := strings.TrimSpace(f.text)
	if f.sending || text == "" {
		return
	}
	if time.Since(f.sentAt) < feedbackEvery {
		f.note, f.failed = "发送太频繁了，请过一分钟再试", true
		return
	}
	f.sentAt = time.Now()
	images := f.images
	paths := make([]string, len(images))
	for i, im := range images {
		paths[i] = im.path
	}
	version := mygo.App.Version()
	f.text, f.images = "", nil
	f.sending, f.note, f.failed = true, "正在后台发送，可以继续使用，发完会提示", false
	go func() {
		err := postFeedback(text, version, paths)
		a.post(func() { a.feedbackSent(text, images, err) })
	}()
}

// feedbackSent says how a message went, and puts it back if it didn't go,
// before whatever has been written since.
func (a *app) feedbackSent(text string, images []feedbackImage, err error) {
	f := &a.feedback
	f.sending = false
	if err == nil {
		f.note, f.failed = "已发送，谢谢你的反馈", false
		a.toast = "反馈已发送"
		return
	}
	log.Printf("feedback: %v", err)
	if f.text != "" {
		text += "\n\n" + f.text
	}
	f.text, f.images = text, append(images, f.images...)
	f.note, f.failed = feedbackFailure(err), true
	a.toast = "反馈发送失败，内容已放回"
}

// feedbackFailure says why a message didn't go, which it keeps.
func feedbackFailure(err error) string {
	var timeout interface{ Timeout() bool }
	switch {
	case errors.Is(err, errFeedbackInactive):
		return "反馈邮箱还没有激活：请先点击邮箱里的 Activate Form 邮件，内容已保留"
	case errors.As(err, &timeout) && timeout.Timeout():
		return "服务器响应太慢，内容已保留，请稍后再试"
	}
	return "发送失败，内容已保留，请稍后再试"
}

// postFeedback posts a message to FormSubmit, the images as its attachments.
func postFeedback(text, version string, images []string) error {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	fields := [][2]string{
		{"_subject", "Cloudy 反馈"},
		{"_template", "table"},
		{"_captcha", "false"},
		{"反馈内容", text},
		{"软件版本", version},
	}
	for _, f := range fields {
		if err := w.WriteField(f[0], f[1]); err != nil {
			return err
		}
	}
	for i, path := range images {
		name := "attachment"
		if i > 0 {
			name = fmt.Sprintf("attachment%d", i+1)
		}
		part, err := w.CreateFormFile(name, filepath.Base(path))
		if err != nil {
			return err
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		_, err = io.Copy(part, file)
		file.Close()
		if err != nil {
			return err
		}
	}
	if err := w.Close(); err != nil {
		return err
	}
	req, err := http.NewRequest("POST", feedbackTo, &body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Referer", feedbackFrom)
	res, err := (&http.Client{Timeout: feedbackWait}).Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	reply, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode >= 300 {
		return fmt.Errorf("%s", res.Status)
	}
	// A page that thanks for the message, or asks for the mailbox to be
	// confirmed first.
	said := strings.ToLower(string(reply))
	if strings.Contains(said, "needs activation") || strings.Contains(said, "activate form") {
		return errFeedbackInactive
	}
	return nil
}
