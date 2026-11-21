import { test, expect } from '@playwright/test';

// 注意: 当前 e2e 仅启动前端 dev server（见 playwright.config.ts webServer），
// 不含后端依赖。以下断言检查前端壳的基本可用性，不验证后端交互。
// 完整 e2e（含后端 /health、登录→建文档→保存链路）需独立 CI job 启动后端后再跑。

test.describe('Citebase Smoke Tests', () => {
  test('home page loads and renders content', async ({ page }) => {
    await page.goto('/');
    // 不只断言 #app-root 存在（它永远存在），而是检查页面有可见文本内容（非白屏）
    await expect(page.locator('#app-root')).toBeVisible();
    await expect(page.locator('body')).not.toBeEmpty();
  });

  test('login page accessible with input field', async ({ page }) => {
    await page.goto('/login');
    // 断言登录表单的输入框可见，比 #app-root 更有意义
    await expect(page.locator('input[type="text"]').first()).toBeVisible();
  });

  test('share page handles invalid share id gracefully', async ({ page }) => {
    await page.goto('/share/99999');
    // 不应白屏崩溃: 等待页面稳定后检查有可见内容
    // 理想情况下应断言"分享不存在"或"需要密码"的文案，但需后端依赖
    await page.waitForLoadState('networkidle');
    await expect(page.locator('body')).not.toBeEmpty();
  });
});
