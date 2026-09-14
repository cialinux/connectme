import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
const {chromium}=await import(process.env.PLAYWRIGHT_MODULE||'/tmp/connectme-browser-tools/node_modules/playwright/index.mjs');
const browser=await chromium.launch({headless:true});
try {
 const page=await browser.newPage();
 await page.route('http://privacy.test/**',route=>route.fulfill({contentType:'application/json',body:JSON.stringify({items:[]})}));
 await page.goto('http://privacy.test/');
 await page.setContent('<div id="notice"></div><button id="logout"></button><form id="password"></form><nav id="navigation"></nav><form id="editor"></form><button id="cancel"></button><h2 id="heading"></h2><button id="refresh"></button><button id="previous"></button><button id="next"></button><table><thead id="columns"></thead><tbody id="rows"></tbody></table><p id="paging"></p>');
 await page.addScriptTag({content:await readFile(new URL('../modules/identity/console.js',import.meta.url),'utf8')});
 await page.evaluate(()=>choose('credentials'));
 const input=page.locator('[name=value]');
 assert.equal(await input.getAttribute('type'),'password');
 assert.equal(await input.getAttribute('autocomplete'),'new-password');
 await input.fill('Test-only-secret');
 assert.equal(await page.locator('body').innerText().then(t=>t.includes('Test-only-secret')),false);
 await page.getByLabel('Importar chave privada sem exibir conteúdo').setInputFiles({name:'test.key',mimeType:'text/plain',buffer:Buffer.from('line1\nline2\n')});
 await page.waitForFunction(()=>document.querySelector('[name=value]').secretValue==='line1\nline2\n');
 assert.equal(await input.getAttribute('type'),'password');
 await input.fill('replacement');
 assert.equal(await input.evaluate(el=>el.secretValue),undefined);
 await page.evaluate(()=>form({id:'dummy',name:'existing',type:'password',enabled:true}));
 assert.equal(await input.inputValue(),'');
 console.log('PASS: masked secrets, no visible content, private key file preserves newlines, edit stays empty');
} finally {await browser.close()}
