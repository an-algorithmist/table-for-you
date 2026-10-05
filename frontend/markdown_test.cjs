const {test}=require('node:test');const assert=require('node:assert/strict');const md=require('./assets/markdown.js');
test('markdown fence, headings, emphasis, lists and tables',()=>{
 const result=md.render('```markdown\n### Meals\n**Soup** and *salad*\n- One\n\n| Dish | Price |\n| --- | --- |\n| Soup | EUR 12 |\n```');
 assert.match(result,/<h3>Meals<\/h3>/);assert.match(result,/<strong>Soup<\/strong>/);assert.match(result,/<em>salad<\/em>/);assert.match(result,/<li>One<\/li>/);assert.match(result,/<table/);assert.ok(!result.includes('```'));
});
test('escapes untrusted HTML and disallows javascript and HTTP links',()=>{
 const result=md.render('<img src=x onerror=alert(1)> [bad](javascript:alert) [local](http://localhost) [menu](https://food.example/menu)');
 assert.ok(!result.includes('<img'));assert.ok(!result.includes('href="javascript:'));assert.ok(!result.includes('href="http:'));assert.match(result,/href="https:\/\/food.example\/menu"/);
});
test('only known citations become links',()=>{const result=md.render('Soup [1] [99]',new Map([[1,{url:'https://food.example'}]]));assert.match(result,/<a /);assert.match(result,/\[99\]/);});
