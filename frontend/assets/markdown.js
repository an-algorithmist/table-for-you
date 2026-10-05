// Safe, dependency-free renderer: provider HTML is always text; only HTTPS links are active.
(function(root){
 const escape=v=>String(v??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
 function safeLink(url,label){try{const u=new URL(url);if(u.protocol!=='https:')return escape(label);return '<a href="'+escape(u.href)+'" target="_blank" rel="noopener noreferrer">'+escape(label)+'</a>';}catch{return escape(label);}}
 function inline(text,refs=new Map()){
  const re=/\[([^\]\n]+)\]\(([^\s)]+)\)|\*\*([^*\n]+)\*\*|\*([^*\n]+)\*|`([^`\n]+)`|\[(\d+)\]/g;
  let out='',at=0;
  for(const m of text.matchAll(re)){out+=escape(text.slice(at,m.index));if(m[1])out+=safeLink(m[2],m[1]);else if(m[3])out+='<strong>'+escape(m[3])+'</strong>';else if(m[4])out+='<em>'+escape(m[4])+'</em>';else if(m[5])out+='<code>'+escape(m[5])+'</code>';else out+=refs.has(Number(m[6]))?safeLink(refs.get(Number(m[6])).url,m[0]):escape(m[0]);at=m.index+m[0].length;}
  return out+escape(text.slice(at));
 }
 function render(value,refs=new Map()){
  const text=String(value??'').trim().replace(/^```(?:markdown|md)?\s*\n([\s\S]*?)\n```\s*$/i,'$1');
  const lines=text.split(/\r?\n/);let out='',list='';const close=()=>{if(list){out+='</'+list+'>';list='';}};
  const cells=l=>l.trim().replace(/^\||\|$/g,'').split('|').map(x=>x.trim());
  for(let i=0;i<lines.length;i++){
   const t=lines[i].trim();if(!t){close();continue;}
   if(t.includes('|')&&i+1<lines.length&&/^\s*\|?\s*:?-{3,}:?\s*(\|\s*:?-{3,}:?\s*)+\|?\s*$/.test(lines[i+1])){
    close();out+='<div class="answer-table-wrap"><table class="answer-table"><thead><tr>'+cells(t).map(c=>'<th scope="col">'+inline(c,refs)+'</th>').join('')+'</tr></thead><tbody>';i++;
    while(i+1<lines.length&&lines[i+1].includes('|')){out+='<tr>'+cells(lines[++i]).map(c=>'<td>'+inline(c,refs)+'</td>').join('')+'</tr>';}
    out+='</tbody></table></div>';continue;
   }
   const h=t.match(/^(#{1,6})\s+(.+)$/),b=t.match(/^[-*•]\s+(.+)$/),n=t.match(/^\d+[.)]\s+(.+)$/);
   if(h){close();out+='<h3>'+inline(h[2],refs)+'</h3>';}
   else if(b||n){const tag=n?'ol':'ul';if(list!==tag){close();list=tag;out+='<'+tag+'>';}out+='<li>'+inline((b||n)[1],refs)+'</li>';}
   else if(/^[-_]{3,}$/.test(t)){close();out+='<hr>';}
   else{close();out+='<p>'+inline(t,refs)+'</p>';}
  }close();return out;
 }
 const api={render,inline};if(typeof module!=='undefined')module.exports=api;root.SafeMarkdown=api;
})(globalThis);
