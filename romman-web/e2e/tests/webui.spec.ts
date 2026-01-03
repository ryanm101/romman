import { test, expect } from '@playwright/test';

test.describe('Dashboard', () => {
    test.beforeEach(async ({ page }) => {
        await page.goto('/');
        // Wait for stats to load
        await page.waitForFunction(() => {
            const el = document.getElementById('stat-systems');
            return el && el.textContent !== '-';
        }, { timeout: 10000 });
    });

    test('displays header with title and refresh button', async ({ page }) => {
        // Check title
        await expect(page.locator('h1')).toContainText('ROM Manager');

        // Check refresh button exists
        const refreshBtn = page.locator('button:has-text("Refresh")');
        await expect(refreshBtn).toBeVisible();
    });

    test('displays stats cards with actual numbers', async ({ page }) => {
        const systemsStat = page.locator('#stat-systems');
        const librariesStat = page.locator('#stat-libraries');
        const releasesStat = page.locator('#stat-releases');

        // Should have actual numbers, not dashes
        await expect(systemsStat).not.toHaveText('-');
        await expect(librariesStat).not.toHaveText('-');
        await expect(releasesStat).not.toHaveText('-');

        // Should be numeric
        const systemsText = await systemsStat.textContent();
        expect(parseInt(systemsText || '0')).toBeGreaterThan(0);
    });

    test('displays library cards', async ({ page }) => {
        // Wait for library cards to load
        await page.waitForSelector('.lib-card', { timeout: 10000 });

        const libCards = page.locator('.lib-card');
        const count = await libCards.count();
        expect(count).toBeGreaterThan(0);
    });

    test('refresh button updates data', async ({ page }) => {
        // Click refresh
        const refreshBtn = page.locator('button:has-text("Refresh")');
        await refreshBtn.click();

        // Check connection status changes during refresh
        const status = page.locator('#connection-status');
        // Wait for it to show "Connected" after refresh
        await expect(status).toContainText('Connected', { timeout: 5000 });
    });
});

test.describe('Library Detail Modal', () => {
    test.beforeEach(async ({ page }) => {
        await page.goto('/');
        // Wait for library cards to load
        await page.waitForSelector('.lib-card', { timeout: 10000 });
    });

    test('opens modal when library card is clicked', async ({ page }) => {
        // Click first library card
        const firstCard = page.locator('.lib-card').first();
        await firstCard.click();

        // Modal should be visible
        const modal = page.locator('#detail-view');
        await expect(modal).toBeVisible();

        // Modal title should contain "Library:"
        const title = page.locator('#modal-title');
        await expect(title).toContainText('Library:');
    });

    test('modal has all filter tabs', async ({ page }) => {
        // Open first library
        await page.locator('.lib-card').first().click();
        await expect(page.locator('#detail-view')).toBeVisible();

        // Check all tabs exist using IDs
        const tabIds = ['tab-matched', 'tab-missing', 'tab-flagged', 'tab-unmatched', 'tab-preferred'];
        for (const tabId of tabIds) {
            const tab = page.locator(`#${tabId}`);
            await expect(tab).toBeVisible();
        }
    });

    test('tabs show counts', async ({ page }) => {
        // Open first library
        await page.locator('.lib-card').first().click();
        await expect(page.locator('#detail-view')).toBeVisible();

        // Wait for counts to load
        await page.waitForTimeout(1000);

        // Check matched tab has a count (format: "Matched (N)")
        const matchedTab = page.locator('#tab-matched');
        const matchedText = await matchedTab.textContent();
        expect(matchedText).toMatch(/Matched \(\d+\)/);
    });

    test('search filters items', async ({ page }) => {
        // Open first library with matches (need to find one)
        await page.locator('.lib-card').first().click();
        await expect(page.locator('#detail-view')).toBeVisible();

        // Type in search box
        const searchInput = page.locator('#game-search');
        await searchInput.fill('Mario');

        // Footer should update to show filtered count
        const footer = page.locator('#modal-footer');
        await expect(footer).toContainText('Showing');
    });

    test('close button closes modal', async ({ page }) => {
        // Open modal
        await page.locator('.lib-card').first().click();
        await expect(page.locator('#detail-view')).toBeVisible();

        // Click close button within the detail modal specifically
        await page.locator('#detail-view button:has-text("Close")').click();

        // Modal should be hidden
        await expect(page.locator('#detail-view')).not.toBeVisible();
    });

    test('Escape key closes modal', async ({ page }) => {
        // Open modal
        await page.locator('.lib-card').first().click();
        await expect(page.locator('#detail-view')).toBeVisible();

        // Press Escape
        await page.keyboard.press('Escape');

        // Modal should be hidden
        await expect(page.locator('#detail-view')).not.toBeVisible();
    });

    test('switching tabs changes content', async ({ page }) => {
        // Open first library
        await page.locator('.lib-card').first().click();
        await expect(page.locator('#detail-view')).toBeVisible();

        // Click Missing tab
        await page.locator('#tab-missing').click();

        // Missing tab should be active
        await expect(page.locator('#tab-missing')).toHaveClass(/active/);
        await expect(page.locator('#tab-matched')).not.toHaveClass(/active/);
    });
});

test.describe('API Endpoints', () => {
    test('stats endpoint returns valid data', async ({ request }) => {
        const response = await request.get('/api/stats');
        expect(response.ok()).toBeTruthy();

        const data = await response.json();
        expect(data).toHaveProperty('totalSystems');
        expect(data).toHaveProperty('totalLibraries');
        expect(data).toHaveProperty('totalReleases');
        expect(typeof data.totalSystems).toBe('number');
    });

    test('libraries endpoint returns valid data', async ({ request }) => {
        const response = await request.get('/api/libraries');
        expect(response.ok()).toBeTruthy();

        const data = await response.json();
        expect(data).toHaveProperty('libraries');
        expect(Array.isArray(data.libraries)).toBeTruthy();
    });

    test('systems endpoint returns valid data', async ({ request }) => {
        const response = await request.get('/api/systems');
        expect(response.ok()).toBeTruthy();

        const data = await response.json();
        expect(data).toHaveProperty('systems');
        expect(Array.isArray(data.systems)).toBeTruthy();
    });

    test('counts endpoint returns valid data', async ({ request }) => {
        // Use a known small library to avoid timeout
        const response = await request.get('/api/counts?library=gb');
        expect(response.ok()).toBeTruthy();

        const data = await response.json();
        expect(data).toHaveProperty('matched');
        expect(data).toHaveProperty('missing');
        expect(data).toHaveProperty('flagged');
        expect(data).toHaveProperty('unmatched');
        expect(data).toHaveProperty('preferred');
    });

    test('details endpoint returns items', async ({ request }) => {
        // First get a library name
        const libsResponse = await request.get('/api/libraries');
        const libsData = await libsResponse.json();

        if (libsData.libraries && libsData.libraries.length > 0) {
            const libName = libsData.libraries[0].name;
            const response = await request.get(`/api/details?library=${libName}&filter=matched`);
            expect(response.ok()).toBeTruthy();

            const data = await response.json();
            expect(data).toHaveProperty('items');
        }
    });

    test('health endpoint returns healthy', async ({ request }) => {
        const response = await request.get('/health');
        expect(response.ok()).toBeTruthy();

        const data = await response.json();
        expect(data.status).toBe('healthy');
    });
});
