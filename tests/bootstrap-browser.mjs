// Run only against a NEW disposable stack on localhost:18080.
import assert from 'node:assert/strict';
const {chromium}=await import(process.env.PLAYWRIGHT_MODULE||'/tmp/connectme-browser-tools/node_modules/playwright/index.mjs');
const browser=await chromium.launch({headless:true});
try {
 const page=await browser.newPage();
 await page.goto('http://127.0.0.1:18080/');
 assert.match(await page.locator('footer').innerText(),/versão \d+\.\d+\.\d+/);
 await page.locator('[name=email]').fill('admin');
 await page.locator('[name=password]').fill('admin');
 await page.getByRole('button',{name:'Entrar',exact:true}).click();
 await page.waitForURL('**/app');
 await page.locator('[name=current_password]').fill('admin');
 await page.locator('[name=new_password]').fill('Isolated-test-only-2026!');
 await page.getByRole('button',{name:'Salvar e entrar novamente',exact:true}).click();
 await page.waitForURL('http://127.0.0.1:18080/');
 console.log('PASS: fresh bootstrap admin/admin, release footer, mandatory password change');
} finally {await browser.close()}
