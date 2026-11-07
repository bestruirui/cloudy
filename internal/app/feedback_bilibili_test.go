package app

import (
	"strings"
	"testing"
)

// Feedback goes to the author's Bilibili messages: what was written, with
// the version of the app and of the system, is copied, and the messages
// open, for the listener to paste it.
func TestFeedbackToBilibili(t *testing.T) {
	oldUID, oldCopy, oldOpen := feedbackBilibiliUID, copyText, openURL
	defer func() { feedbackBilibiliUID, copyText, openURL = oldUID, oldCopy, oldOpen }()
	feedbackBilibiliUID = 12345
	var copied, opened string
	copyText = func(s string) error { copied = s; return nil }
	openURL = func(u string) error { opened = u; return nil }

	a := newTestApp()
	a.feedback.text = "  桌面歌词偶尔会闪  "
	a.feedbackToBilibili()
	if !strings.HasPrefix(copied, "桌面歌词偶尔会闪\n\n——\nCloudy ") || !strings.Contains(copied, osVersion()) {
		t.Errorf("copied %q", copied)
	}
	if opened != "https://message.bilibili.com/#/whisper/mid12345" {
		t.Errorf("opened %q", opened)
	}
	if a.feedback.failed || a.feedback.text == "" {
		t.Errorf("note %q failed %v, text kept %q", a.feedback.note, a.feedback.failed, a.feedback.text)
	}
	t.Logf("system: %s", osVersion())
}
