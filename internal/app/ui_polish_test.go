package app

import (
	"strings"
	"testing"
)

func TestEmbeddedUIResponsiveAccessibilityPolishContract(t *testing.T) {
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()

	html := embeddedUIAsset(t, service, "/", "text/html")
	javascript := embeddedUIAsset(t, service, "/ui/app.js", "text/javascript")
	styles := embeddedUIAsset(t, service, "/ui/app.css", "text/css")

	for _, value := range []string{
		`aria-controls="import-project-file" aria-describedby="project-import-status"`,
		`id="import-project-file" name="project-import-file"`,
		`aria-label="가져올 프로젝트 설정 파일"`,
		`id="project-import-status" class="visually-hidden" role="status" aria-live="polite" aria-atomic="true"`,
	} {
		if !strings.Contains(html, value) {
			t.Errorf("embedded UI import accessibility contract missing %q", value)
		}
	}

	for _, value := range []string{
		`if (id === "main-content") element.classList.add("route-focus");`,
		`<div class="table-wrap" tabindex="0" role="region"`,
		`aria-label="활동 기록 표. 좌우로 스크롤할 수 있습니다."`,
		`aria-describedby="activity-table-scroll-hint"`,
		`id="activity-table-scroll-hint" class="table-scroll-hint"`,
		`["ArrowLeft", "ArrowRight", "Home", "End"]`,
		`const hasObservedTarget = projectID => targetOptions().some`,
		`data-repository-refresh`,
		`targetSelect.disabled = !targets.length || Boolean(state.qualityRunPending);`,
		`projectImportFile?.addEventListener("cancel"`,
		`프로젝트 설정 파일을 선택하지 않았습니다.`,
	} {
		if !strings.Contains(javascript, value) {
			t.Errorf("embedded UI behavior contract missing %q", value)
		}
	}

	for _, value := range []string{
		`min-width: 0;`,
		`:focus-visible`,
		`outline: 2px solid var(--accent);`,
		`.table-wrap:focus-visible`,
		`.table-scroll-hint`,
		`.quality-work-surface`,
		`.advanced-work`,
		`.provider-card`,
	} {
		if !strings.Contains(styles, value) {
			t.Errorf("embedded UI style contract missing %q", value)
		}
	}
	if strings.Contains(styles, "min-width: 320px") {
		t.Fatal("embedded UI must not impose a 320px page minimum")
	}
	if strings.Contains(styles, "overflow-x: hidden") {
		t.Fatal("embedded UI must not hide page overflow to mask narrow-layout clipping")
	}
	if strings.Contains(styles, "main:focus-visible") || strings.Contains(styles, "main.route-focus:focus") {
		t.Fatal("route focus must not draw a full main-content box")
	}
	if strings.Contains(javascript, `href="#projects">프로젝트 등록으로 이동</a>`) {
		t.Fatal("empty Home must not duplicate the onboarding project-registration CTA")
	}
}
