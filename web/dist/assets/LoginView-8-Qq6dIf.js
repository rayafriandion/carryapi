import{$ as e,$t as t,En as n,Ft as r,Hn as i,In as a,It as o,Lt as s,Nt as c,Wn as l,Yt as u,an as d,cn as f,en as p,et as m,in as h,n as g,on as _,qt as v,r as y,rn as b,t as x,tn as S,u as C,v as w,yn as T}from"./http-BFtIegGs.js";import{t as E}from"./_plugin-vue_export-helper-CEWy_VHC.js";import{n as D,t as O}from"./FormItem-DabG5Utk.js";import{n as k,t as A}from"./Tabs-DwhZho18.js";import{t as j}from"./Input-Dxn25Ia1.js";import{t as M}from"./use-message-BqNUsXIc.js";import{o as N,r as P,t as F}from"./index-CJR9ezvC.js";function I(e){let{textColor1:t,dividerColor:n,fontWeightStrong:r}=e;return{textColor:t,color:n,fontWeight:r}}var L={name:`Divider`,common:C,self:I},R=c(`divider`,`
 position: relative;
 display: flex;
 width: 100%;
 box-sizing: border-box;
 font-size: 16px;
 color: var(--n-text-color);
 transition:
 color .3s var(--n-bezier),
 background-color .3s var(--n-bezier);
`,[s(`vertical`,`
 margin-top: 24px;
 margin-bottom: 24px;
 `,[s(`no-title`,`
 display: flex;
 align-items: center;
 `)]),r(`title`,`
 display: flex;
 align-items: center;
 margin-left: 12px;
 margin-right: 12px;
 white-space: nowrap;
 font-weight: var(--n-font-weight);
 `),o(`title-position-left`,[r(`line`,[o(`left`,{width:`28px`})])]),o(`title-position-right`,[r(`line`,[o(`right`,{width:`28px`})])]),o(`dashed`,[r(`line`,`
 background-color: #0000;
 height: 0px;
 width: 100%;
 border-style: dashed;
 border-width: 1px 0 0;
 `)]),o(`vertical`,`
 display: inline-block;
 height: 1em;
 margin: 0 8px;
 vertical-align: middle;
 width: 1px;
 `),r(`line`,`
 border: none;
 transition: background-color .3s var(--n-bezier), border-color .3s var(--n-bezier);
 height: 1px;
 width: 100%;
 margin: 0;
 `),s(`dashed`,[r(`line`,{backgroundColor:`var(--n-color)`})]),o(`dashed`,[r(`line`,{borderColor:`var(--n-color)`})]),o(`vertical`,{backgroundColor:`var(--n-color)`})]),z=Object.assign(Object.assign({},w.props),{titlePlacement:{type:String,default:`center`},dashed:Boolean,vertical:Boolean}),B=_({name:`Divider`,props:z,setup(n){let{mergedClsPrefixRef:r,inlineThemeDisabled:i}=m(n),a=w(`Divider`,`-divider`,R,L,n,r),o=t(()=>{let{common:{cubicBezierEaseInOut:e},self:{color:t,textColor:n,fontWeight:r}}=a.value;return{"--n-bezier":e,"--n-color":t,"--n-text-color":n,"--n-font-weight":r}}),s=i?e(`divider`,void 0,o,n):void 0;return{mergedClsPrefix:r,cssVars:i?void 0:o,themeClass:s?.themeClass,onRender:s?.onRender}},render(){var e;let{$slots:t,titlePlacement:n,vertical:r,dashed:i,cssVars:a,mergedClsPrefix:o}=this;return(e=this.onRender)==null||e.call(this),f(`div`,{role:`separator`,class:[`${o}-divider`,this.themeClass,{[`${o}-divider--vertical`]:r,[`${o}-divider--no-title`]:!t.default,[`${o}-divider--dashed`]:i,[`${o}-divider--title-position-${n}`]:t.default&&n}],style:a},r?null:f(`div`,{class:`${o}-divider__line ${o}-divider__line--left`}),!r&&t.default?f(u,null,f(`div`,{class:`${o}-divider__title`},this.$slots),f(`div`,{class:`${o}-divider__line ${o}-divider__line--right`})):null)}}),V={class:`login-page`},H={class:`oauth-row`},U={href:`/api/auth/oauth/discord`},W={href:`/api/auth/oauth/x`},G=E(_({__name:`LoginView`,setup(e){let t=P(),r=M(),o=F(),s=a(`login`),c=a(!1),u=a(!1),f=a(!1),m=a(!1),_=a({email:``,password:``,code:``}),C=a({email:``,password:``}),w=a(``),E=a(),I=a(),L={email:{required:!0,message:`请输入邮箱`,trigger:`blur`},password:{required:!0,message:`请输入密码`,trigger:`blur`}},R={email:{required:!0,message:`请输入邮箱`,trigger:`blur`},password:{required:!0,message:`请输入密码`,trigger:`blur`}};async function z(){if(m.value)return G();c.value=!0;try{(await o.login(_.value.email,_.value.password)).requires2fa?(m.value=!0,r.info(`该账户已开启两步验证，请输入 TOTP 验证码`)):(r.success(`登录成功`),t.push(`/`))}catch(e){r.error(x(e))}finally{c.value=!1}}async function G(){c.value=!0;try{await o.complete2FA(_.value.email,_.value.code),r.success(`登录成功`),t.push(`/`)}catch(e){r.error(x(e))}finally{c.value=!1}}async function K(){u.value=!0;try{await o.register(C.value.email,C.value.password),r.success(`账户创建成功，请登录`),_.value.email=C.value.email,_.value.password=C.value.password,m.value=!1,s.value=`login`}catch(e){r.error(x(e))}finally{u.value=!1}}async function q(){if(!w.value){r.warning(`请输入邮箱`);return}f.value=!0;try{let{publicKey:e,session_key:n}=(await g.post(`/api/auth/passkey/login/begin`,{},{params:{email:w.value}})).data,i=await navigator.credentials.get({publicKey:e}),a=await g.post(`/api/auth/passkey/login/finish`,i,{params:{email:w.value,session_key:n}});o.setUser(a.data.user),r.success(`登录成功`),t.push(`/`)}catch(e){r.error(x(e))}finally{f.value=!1}}return(e,t)=>(T(),b(`div`,V,[d(i(N),{class:`login-card`,bordered:!1},{default:n(()=>[t[13]||=p(`div`,{class:`login-title`},`carryAPI 控制台`,-1),d(i(A),{value:s.value,"onUpdate:value":t[7]||=e=>s.value=e,type:`line`,animated:``},{default:n(()=>[d(i(k),{name:`login`,tab:`登录`},{default:n(()=>[d(i(D),{ref_key:`loginFormRef`,ref:E,model:_.value,rules:L},{default:n(()=>[d(i(O),{label:`邮箱`,path:`email`},{default:n(()=>[d(i(j),{value:_.value.email,"onUpdate:value":t[0]||=e=>_.value.email=e,placeholder:`you@example.com`},null,8,[`value`])]),_:1}),m.value?(T(),S(i(O),{key:1,label:`TOTP 验证码`,path:`code`},{default:n(()=>[d(i(j),{value:_.value.code,"onUpdate:value":t[2]||=e=>_.value.code=e,placeholder:`6 位验证码`,onKeydown:v(G,[`enter`])},null,8,[`value`])]),_:1})):(T(),S(i(O),{key:0,label:`密码`,path:`password`},{default:n(()=>[d(i(j),{value:_.value.password,"onUpdate:value":t[1]||=e=>_.value.password=e,type:`password`,"show-password-on":`click`,placeholder:`密码`,onKeydown:v(z,[`enter`])},null,8,[`value`])]),_:1})),d(i(y),{type:`primary`,block:``,loading:c.value,onClick:t[3]||=e=>m.value?G():z()},{default:n(()=>[h(l(m.value?`验证`:`登录`),1)]),_:1},8,[`loading`])]),_:1},8,[`model`]),p(`div`,H,[p(`a`,U,[d(i(y),{type:`info`,tertiary:``},{default:n(()=>[...t[8]||=[h(`Discord 登录`,-1)]]),_:1})]),p(`a`,W,[d(i(y),{type:`default`,tertiary:``},{default:n(()=>[...t[9]||=[h(`X 登录`,-1)]]),_:1})])]),d(i(B),{"title-placement":`left`},{default:n(()=>[...t[10]||=[h(`或`,-1)]]),_:1}),d(i(D),{class:`passkey-form`},{default:n(()=>[d(i(O),{label:`Passkey 邮箱`},{default:n(()=>[d(i(j),{value:w.value,"onUpdate:value":t[4]||=e=>w.value=e,placeholder:`you@example.com`,onKeydown:v(q,[`enter`])},null,8,[`value`])]),_:1}),d(i(y),{block:``,loading:f.value,onClick:q},{default:n(()=>[...t[11]||=[h(`使用 Passkey 登录`,-1)]]),_:1},8,[`loading`])]),_:1})]),_:1}),d(i(k),{name:`register`,tab:`注册`},{default:n(()=>[d(i(D),{ref_key:`registerFormRef`,ref:I,model:C.value,rules:R},{default:n(()=>[d(i(O),{label:`邮箱`,path:`email`},{default:n(()=>[d(i(j),{value:C.value.email,"onUpdate:value":t[5]||=e=>C.value.email=e,placeholder:`you@example.com`},null,8,[`value`])]),_:1}),d(i(O),{label:`密码`,path:`password`},{default:n(()=>[d(i(j),{value:C.value.password,"onUpdate:value":t[6]||=e=>C.value.password=e,type:`password`,"show-password-on":`click`,placeholder:`至少 8 位`},null,8,[`value`])]),_:1}),d(i(y),{type:`primary`,block:``,loading:u.value,onClick:K},{default:n(()=>[...t[12]||=[h(`创建账户`,-1)]]),_:1},8,[`loading`])]),_:1},8,[`model`])]),_:1})]),_:1},8,[`value`])]),_:1})]))}}),[[`__scopeId`,`data-v-f79f5be5`]]);export{G as default};