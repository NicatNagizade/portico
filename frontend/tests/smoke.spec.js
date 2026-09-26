import { expect, test } from '@playwright/test'

test('loads the app shell', async ({ page }) => {
  await page.goto('/')
  await expect(page.getByRole('heading', { name: 'Portico' })).toBeVisible()
  await expect(page.getByRole('link', { name: /Connections/ })).toBeVisible()
  await expect(page.getByRole('link', { name: /Sync Jobs/ })).toBeVisible()
  await expect(page).toHaveURL(/\/connections/)
})
