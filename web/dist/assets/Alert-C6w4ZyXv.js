import{$ as e,$t as t,Ft as n,In as r,It as i,Mt as a,Nt as o,Ot as s,Rt as c,Tt as l,cn as u,ct as d,dn as f,et as p,g as m,on as h,ot as g,p as _,u as v,v as y,wt as b,x}from"./http-BFtIegGs.js";import{c as S,d as C,f as w,l as T,s as E,u as D}from"./index-CJR9ezvC.js";var O={iconMargin:`11px 8px 0 12px`,iconMarginRtl:`11px 12px 0 8px`,iconSize:`24px`,closeIconSize:`16px`,closeSize:`20px`,closeMargin:`13px 14px 0 0`,closeMarginRtl:`13px 0 0 14px`,padding:`13px`};function k(e){let{lineHeight:t,borderRadius:n,fontWeightStrong:r,baseColor:i,dividerColor:a,actionColor:o,textColor1:s,textColor2:c,closeColorHover:u,closeColorPressed:d,closeIconColor:f,closeIconColorHover:p,closeIconColorPressed:m,infoColor:h,successColor:g,warningColor:_,errorColor:v,fontSize:y}=e;return Object.assign(Object.assign({},O),{fontSize:y,lineHeight:t,titleFontWeight:r,borderRadius:n,border:`1px solid ${a}`,color:o,titleTextColor:s,iconColor:c,contentTextColor:c,closeBorderRadius:n,closeColorHover:u,closeColorPressed:d,closeIconColor:f,closeIconColorHover:p,closeIconColorPressed:m,borderInfo:`1px solid ${l(i,b(h,{alpha:.25}))}`,colorInfo:l(i,b(h,{alpha:.08})),titleTextColorInfo:s,iconColorInfo:h,contentTextColorInfo:c,closeColorHoverInfo:u,closeColorPressedInfo:d,closeIconColorInfo:f,closeIconColorHoverInfo:p,closeIconColorPressedInfo:m,borderSuccess:`1px solid ${l(i,b(g,{alpha:.25}))}`,colorSuccess:l(i,b(g,{alpha:.08})),titleTextColorSuccess:s,iconColorSuccess:g,contentTextColorSuccess:c,closeColorHoverSuccess:u,closeColorPressedSuccess:d,closeIconColorSuccess:f,closeIconColorHoverSuccess:p,closeIconColorPressedSuccess:m,borderWarning:`1px solid ${l(i,b(_,{alpha:.33}))}`,colorWarning:l(i,b(_,{alpha:.08})),titleTextColorWarning:s,iconColorWarning:_,contentTextColorWarning:c,closeColorHoverWarning:u,closeColorPressedWarning:d,closeIconColorWarning:f,closeIconColorHoverWarning:p,closeIconColorPressedWarning:m,borderError:`1px solid ${l(i,b(v,{alpha:.25}))}`,colorError:l(i,b(v,{alpha:.08})),titleTextColorError:s,iconColorError:v,contentTextColorError:c,closeColorHoverError:u,closeColorPressedError:d,closeIconColorError:f,closeIconColorHoverError:p,closeIconColorPressedError:m})}var A={name:`Alert`,common:v,self:k},j=o(`alert`,`
 line-height: var(--n-line-height);
 border-radius: var(--n-border-radius);
 position: relative;
 transition: background-color .3s var(--n-bezier);
 background-color: var(--n-color);
 text-align: start;
 word-break: break-word;
`,[n(`border`,`
 border-radius: inherit;
 position: absolute;
 left: 0;
 right: 0;
 top: 0;
 bottom: 0;
 transition: border-color .3s var(--n-bezier);
 border: var(--n-border);
 pointer-events: none;
 `),i(`closable`,[o(`alert-body`,[n(`title`,`
 padding-right: 24px;
 `)])]),n(`icon`,{color:`var(--n-icon-color)`}),o(`alert-body`,{padding:`var(--n-padding)`},[n(`title`,{color:`var(--n-title-text-color)`}),n(`content`,{color:`var(--n-content-text-color)`})]),E({originalTransition:`transform .3s var(--n-bezier)`,enterToProps:{transform:`scale(1)`},leaveToProps:{transform:`scale(0.9)`}}),n(`icon`,`
 position: absolute;
 left: 0;
 top: 0;
 align-items: center;
 justify-content: center;
 display: flex;
 width: var(--n-icon-size);
 height: var(--n-icon-size);
 font-size: var(--n-icon-size);
 margin: var(--n-icon-margin);
 `),n(`close`,`
 transition:
 color .3s var(--n-bezier),
 background-color .3s var(--n-bezier);
 position: absolute;
 right: 0;
 top: 0;
 margin: var(--n-close-margin);
 `),i(`show-icon`,[o(`alert-body`,{paddingLeft:`calc(var(--n-icon-margin-left) + var(--n-icon-size) + var(--n-icon-margin-right))`})]),i(`right-adjust`,[o(`alert-body`,{paddingRight:`calc(var(--n-close-size) + var(--n-padding) + 2px)`})]),o(`alert-body`,`
 border-radius: var(--n-border-radius);
 transition: border-color .3s var(--n-bezier);
 `,[n(`title`,`
 transition: color .3s var(--n-bezier);
 font-size: 16px;
 line-height: 19px;
 font-weight: var(--n-title-font-weight);
 `,[a(`& +`,[n(`content`,{marginTop:`9px`})])]),n(`content`,{transition:`color .3s var(--n-bezier)`,fontSize:`var(--n-font-size)`})]),n(`icon`,{transition:`color .3s var(--n-bezier)`})]),M=Object.assign(Object.assign({},y.props),{title:String,showIcon:{type:Boolean,default:!0},type:{type:String,default:`default`},bordered:{type:Boolean,default:!0},closable:Boolean,onClose:Function,onAfterLeave:Function,onAfterHide:Function}),N=h({name:`Alert`,inheritAttrs:!1,props:M,slots:Object,setup(n){let{mergedClsPrefixRef:i,mergedBorderedRef:a,inlineThemeDisabled:o,mergedRtlRef:l}=p(n),u=y(`Alert`,`-alert`,j,A,n,i),d=x(`Alert`,l,i),f=t(()=>{let{common:{cubicBezierEaseInOut:e},self:t}=u.value,{fontSize:r,borderRadius:i,titleFontWeight:a,lineHeight:o,iconSize:l,iconMargin:d,iconMarginRtl:f,closeIconSize:p,closeBorderRadius:m,closeSize:h,closeMargin:g,closeMarginRtl:_,padding:v}=t,{type:y}=n,{left:b,right:x}=s(d);return{"--n-bezier":e,"--n-color":t[c(`color`,y)],"--n-close-icon-size":p,"--n-close-border-radius":m,"--n-close-color-hover":t[c(`closeColorHover`,y)],"--n-close-color-pressed":t[c(`closeColorPressed`,y)],"--n-close-icon-color":t[c(`closeIconColor`,y)],"--n-close-icon-color-hover":t[c(`closeIconColorHover`,y)],"--n-close-icon-color-pressed":t[c(`closeIconColorPressed`,y)],"--n-icon-color":t[c(`iconColor`,y)],"--n-border":t[c(`border`,y)],"--n-title-text-color":t[c(`titleTextColor`,y)],"--n-content-text-color":t[c(`contentTextColor`,y)],"--n-line-height":o,"--n-border-radius":i,"--n-font-size":r,"--n-title-font-weight":a,"--n-icon-size":l,"--n-icon-margin":d,"--n-icon-margin-rtl":f,"--n-close-size":h,"--n-close-margin":g,"--n-close-margin-rtl":_,"--n-padding":v,"--n-icon-margin-left":b,"--n-icon-margin-right":x}}),m=o?e(`alert`,t(()=>n.type[0]),f,n):void 0,h=r(!0),g=()=>{let{onAfterLeave:e,onAfterHide:t}=n;e&&e(),t&&t()};return{rtlEnabled:d,mergedClsPrefix:i,mergedBordered:a,visible:h,handleCloseClick:()=>{Promise.resolve(n.onClose?.call(n)).then(e=>{e!==!1&&(h.value=!1)})},handleAfterLeave:()=>{g()},mergedTheme:u,cssVars:o?void 0:f,themeClass:m?.themeClass,onRender:m?.onRender}},render(){var e;return(e=this.onRender)==null||e.call(this),u(_,{onAfterLeave:this.handleAfterLeave},{default:()=>{let{mergedClsPrefix:e,$slots:t}=this,n={class:[`${e}-alert`,this.themeClass,this.closable&&`${e}-alert--closable`,this.showIcon&&`${e}-alert--show-icon`,!this.title&&this.closable&&`${e}-alert--right-adjust`,this.rtlEnabled&&`${e}-alert--rtl`],style:this.cssVars,role:`alert`};return this.visible?u(`div`,Object.assign({},f(this.$attrs,n)),this.closable&&u(S,{clsPrefix:e,class:`${e}-alert__close`,onClick:this.handleCloseClick}),this.bordered&&u(`div`,{class:`${e}-alert__border`}),this.showIcon&&u(`div`,{class:`${e}-alert__icon`,"aria-hidden":`true`},g(t.icon,()=>[u(m,{clsPrefix:e},{default:()=>{switch(this.type){case`success`:return u(D,null);case`info`:return u(C,null);case`warning`:return u(T,null);case`error`:return u(w,null);default:return null}}})])),u(`div`,{class:[`${e}-alert-body`,this.mergedBordered&&`${e}-alert-body--bordered`]},d(t.header,t=>{let n=t||this.title;return n?u(`div`,{class:`${e}-alert-body__title`},n):null}),t.default&&u(`div`,{class:`${e}-alert-body__content`},t))):null}})}});export{N as t};