import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
const {chromium}=await import(process.env.PLAYWRIGHT_MODULE||'/tmp/connectme-browser-tools/node_modules/playwright/index.mjs');
const browser=await chromium.launch({headless:true});
try {
 const page=await browser.newPage();
 let request, reject=true;
 await page.route('http://users.test/**',async route=>{
  if(route.request().method()==='POST') {
   request=route.request().postDataJSON();
   return route.fulfill({status:reject?400:201,contentType:'application/json',body:JSON.stringify(reject?{error:{detail:'Utilizador já existente'}}:{id:'test'})});
  }
  return route.fulfill({contentType:'application/json',body:JSON.stringify({items:[]})});
 });
 await page.goto('http://users.test/');
 await page.setContent('<p id="notice"></p><button id="logout"></button><form id="password"></form><nav id="navigation"></nav><form id="editor"></form><button id="cancel"></button><h2 id="heading"></h2><button id="refresh"></button><button id="previous"></button><button id="next"></button><table><thead id="columns"></thead><tbody id="rows"></tbody></table><p id="paging"></p>');
 await page.addScriptTag({content:await readFile(new URL('../modules/identity/console.js',import.meta.url),'utf8')});
 await page.evaluate(()=>choose('users'));
 await page.locator('[name=email]').fill('operador');
 await page.locator('[name=display_name]').fill('Operador');
 await page.locator('[name=password]').fill('short');
 await page.locator('#editor button').click();
 assert.match(await page.locator('#editor-feedback').innerText(),/12 caracteres/);
 assert.equal(request,undefined);
 await page.locator('[name=password]').fill('test-password-123');
 await page.locator('#editor button').click();
 await page.waitForFunction(()=>document.querySelector('#editor-feedback').textContent==='Utilizador já existente');
 assert.equal(request.email,'operador');
 assert.equal(await page.locator('#editor button').isEnabled(),true);
 reject=false;
 await page.locator('#editor button').click();
 await page.waitForFunction(()=>document.querySelector('#editor-feedback').textContent==='Alterações salvas.');
 assert.equal(await page.locator('[name=password]').inputValue(),'');
 console.log('PASS: validation feedback, username submitted, API errors visible, successful form reset');
} finally {await browser.close()}
