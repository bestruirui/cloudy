package app

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/egoist/mygo"
)

// feedbackMessage is what a listener sends: what they wrote, and what the
// author needs to know to look into it.
func feedbackMessage(text string) string {
	return fmt.Sprintf("%s\n\n——\nCloudy %s · %s · %s", strings.TrimSpace(text), mygo.App.Version(), osVersion(), runtime.GOARCH)
}

// feedbackPage is the author's private messages on Bilibili.
func feedbackPage() string {
	return fmt.Sprintf("https://message.bilibili.com/#/whisper/mid%d", feedbackBilibiliUID)
}

// openURL opens a page in the browser; tests replace it.
var openURL = mygo.Shell.OpenExternal

// feedbackToBilibili puts the message on the clipboard and opens the
// author's private messages, for the listener to paste it there: Bilibili
// lets no app send one, and its messages reach the author wherever the
// listener is, as mail through a form did not.
func (a *app) feedbackToBilibili() {
	f := &a.feedback
	if strings.TrimSpace(f.text) == "" {
		return
	}
	if err := copyText(feedbackMessage(f.text)); err != nil {
		f.note, f.failed = "无法复制到剪贴板，请手动复制上面的内容", true
		return
	}
	if err := openURL(feedbackPage()); err != nil {
		f.note, f.failed = "已复制，但无法打开浏览器："+feedbackPage(), true
		return
	}
	f.note, f.failed = "已复制反馈内容，在打开的 B 站私信里粘贴发送即可；图片请在私信里添加", false
	a.toast = "反馈内容已复制"
}

// feedbackIntro says where what is written goes.
func feedbackIntro() string {
	if feedbackBilibiliUID == 0 {
		return "问题、想法或建议都可以写在这里，会连同图片和软件版本一起发给nya"
	}
	return "问题、想法或建议都可以写在这里。点「发到 B 站私信」会复制内容、软件和系统版本，并打开 nya 的私信页面，粘贴发送即可"
}
