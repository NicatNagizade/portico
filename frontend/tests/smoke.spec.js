import { expect, test } from '@playwright/test'

test('loads the landing page', async ({ page }) => {
  await page.goto('/')
  await expect(page.getByRole('heading', { name: 'Portico' })).toBeVisible()
  await expect(page.getByRole('link', { name: 'Open admin' }).first()).toBeVisible()
  await expect(page.getByRole('link', { name: 'Explore data' })).toBeVisible()
})

test('opens the admin shell from landing', async ({ page }) => {
  await page.goto('/')
  await page.getByRole('link', { name: 'Open admin' }).first().click()
  await expect(page).toHaveURL(/\/connections/)
  await expect(page.getByRole('link', { name: /Connections/ })).toBeVisible()
  await expect(page.getByRole('link', { name: /Sync Jobs/ })).toBeVisible()
})
