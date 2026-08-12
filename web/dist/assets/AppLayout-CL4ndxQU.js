import{$ as e,$t as t,Bn as n,Cn as r,En as i,Ft as a,Hn as o,In as s,It as c,Lt as l,Mt as u,Nt as d,Tn as f,Tt as p,Wn as m,Yt as h,_ as g,_t as _,an as v,bn as y,cn as b,dn as x,en as S,et as C,g as w,in as T,l as E,lt as D,mt as ee,on as O,p as te,r as k,s as A,tn as j,u as M,un as N,ut as P,v as F,wt as I,xt as L,yn as ne,yt as R}from"./http-BFtIegGs.js";import{c as z,f as re,i as ie,n as ae,r as oe,t as se,u as ce}from"./Dropdown-Bku9IxDh.js";import{t as le}from"./misc-DDs3MKLt.js";import{n as B}from"./fade-in-scale-up.cssr-B6GsfmmX.js";import{t as V}from"./render-B6RXEyT6.js";import{n as H,t as ue}from"./_plugin-vue_export-helper-CEWy_VHC.js";import{t as de}from"./use-compitable-qn4jAEEw.js";import{o as U}from"./get-b1QfMBHx.js";import{n as fe,r as pe,s as me,t as he}from"./index-Dgxo2YX3.js";var ge=O({name:`ChevronDownFilled`,render(){return b(`svg`,{viewBox:`0 0 16 16`,fill:`none`,xmlns:`http://www.w3.org/2000/svg`},b(`path`,{d:`M3.20041 5.73966C3.48226 5.43613 3.95681 5.41856 4.26034 5.70041L8 9.22652L11.7397 5.70041C12.0432 5.41856 12.5177 5.43613 12.7996 5.73966C13.0815 6.0432 13.0639 6.51775 12.7603 6.7996L8.51034 10.7996C8.22258 11.0668 7.77743 11.0668 7.48967 10.7996L3.23966 6.7996C2.93613 6.51775 2.91856 6.0432 3.20041 5.73966Z`,fill:`currentColor`}))}});function _e(e){let{baseColor:t,textColor2:n,bodyColor:r,cardColor:i,dividerColor:a,actionColor:o,scrollbarColor:s,scrollbarColorHover:c,invertedColor:l}=e;return{textColor:n,textColorInverted:`#FFF`,color:r,colorEmbedded:o,headerColor:i,headerColorInverted:l,footerColor:o,footerColorInverted:l,headerBorderColor:a,headerBorderColorInverted:l,footerBorderColor:a,footerBorderColorInverted:l,siderBorderColor:a,siderBorderColorInverted:l,siderColor:i,siderColorInverted:l,siderToggleButtonBorder:`1px solid ${a}`,siderToggleButtonColor:t,siderToggleButtonIconColor:n,siderToggleButtonIconColorInverted:n,siderToggleBarColor:p(r,s),siderToggleBarColorHover:p(r,c),__invertScrollbar:`true`}}var W=g({name:`Layout`,common:M,peers:{Scrollbar:E},self:_e});function ve(e,t,n,r){return{itemColorHoverInverted:`#0000`,itemColorActiveInverted:t,itemColorActiveHoverInverted:t,itemColorActiveCollapsedInverted:t,itemTextColorInverted:e,itemTextColorHoverInverted:n,itemTextColorChildActiveInverted:n,itemTextColorChildActiveHoverInverted:n,itemTextColorActiveInverted:n,itemTextColorActiveHoverInverted:n,itemTextColorHorizontalInverted:e,itemTextColorHoverHorizontalInverted:n,itemTextColorChildActiveHorizontalInverted:n,itemTextColorChildActiveHoverHorizontalInverted:n,itemTextColorActiveHorizontalInverted:n,itemTextColorActiveHoverHorizontalInverted:n,itemIconColorInverted:e,itemIconColorHoverInverted:n,itemIconColorActiveInverted:n,itemIconColorActiveHoverInverted:n,itemIconColorChildActiveInverted:n,itemIconColorChildActiveHoverInverted:n,itemIconColorCollapsedInverted:e,itemIconColorHorizontalInverted:e,itemIconColorHoverHorizontalInverted:n,itemIconColorActiveHorizontalInverted:n,itemIconColorActiveHoverHorizontalInverted:n,itemIconColorChildActiveHorizontalInverted:n,itemIconColorChildActiveHoverHorizontalInverted:n,arrowColorInverted:e,arrowColorHoverInverted:n,arrowColorActiveInverted:n,arrowColorActiveHoverInverted:n,arrowColorChildActiveInverted:n,arrowColorChildActiveHoverInverted:n,groupTextColorInverted:r}}function ye(e){let{borderRadius:t,textColor3:n,primaryColor:r,textColor2:i,textColor1:a,fontSize:o,dividerColor:s,hoverColor:c,primaryColorHover:l}=e;return Object.assign({borderRadius:t,color:`#0000`,groupTextColor:n,itemColorHover:c,itemColorActive:I(r,{alpha:.1}),itemColorActiveHover:I(r,{alpha:.1}),itemColorActiveCollapsed:I(r,{alpha:.1}),itemTextColor:i,itemTextColorHover:i,itemTextColorActive:r,itemTextColorActiveHover:r,itemTextColorChildActive:r,itemTextColorChildActiveHover:r,itemTextColorHorizontal:i,itemTextColorHoverHorizontal:l,itemTextColorActiveHorizontal:r,itemTextColorActiveHoverHorizontal:r,itemTextColorChildActiveHorizontal:r,itemTextColorChildActiveHoverHorizontal:r,itemIconColor:a,itemIconColorHover:a,itemIconColorActive:r,itemIconColorActiveHover:r,itemIconColorChildActive:r,itemIconColorChildActiveHover:r,itemIconColorCollapsed:a,itemIconColorHorizontal:a,itemIconColorHoverHorizontal:l,itemIconColorActiveHorizontal:r,itemIconColorActiveHoverHorizontal:r,itemIconColorChildActiveHorizontal:r,itemIconColorChildActiveHoverHorizontal:r,itemHeight:`42px`,arrowColor:i,arrowColorHover:i,arrowColorActive:r,arrowColorActiveHover:r,arrowColorChildActive:r,arrowColorChildActiveHover:r,colorInverted:`#0000`,borderColorHorizontal:`#0000`,fontSize:o,dividerColor:s},ve(`#BBB`,r,`#FFF`,`#AAA`))}var be=g({name:`Menu`,common:M,peers:{Tooltip:oe,Dropdown:ie},self:ye}),xe=R(`n-layout-sider`),G={type:String,default:`static`},Se=d(`layout`,`
 color: var(--n-text-color);
 background-color: var(--n-color);
 box-sizing: border-box;
 position: relative;
 z-index: auto;
 flex: auto;
 overflow: hidden;
 transition:
 box-shadow .3s var(--n-bezier),
 background-color .3s var(--n-bezier),
 color .3s var(--n-bezier);
`,[d(`layout-scroll-container`,`
 overflow-x: hidden;
 box-sizing: border-box;
 height: 100%;
 `),c(`absolute-positioned`,`
 position: absolute;
 left: 0;
 right: 0;
 top: 0;
 bottom: 0;
 `)]),Ce={embedded:Boolean,position:G,nativeScrollbar:{type:Boolean,default:!0},scrollbarProps:Object,onScroll:Function,contentClass:String,contentStyle:{type:[String,Object],default:``},hasSider:Boolean,siderPlacement:{type:String,default:`left`}},we=R(`n-layout`);function Te(n){return O({name:n?`LayoutContent`:`Layout`,props:Object.assign(Object.assign({},F.props),Ce),setup(n){let r=s(null),i=s(null),{mergedClsPrefixRef:a,inlineThemeDisabled:o}=C(n),c=F(`Layout`,`-layout`,Se,W,n,a);function l(e,t){if(n.nativeScrollbar){let{value:n}=r;n&&(t===void 0?n.scrollTo(e):n.scrollTo(e,t))}else{let{value:n}=i;n&&n.scrollTo(e,t)}}y(we,n);let u=0,d=0,f=e=>{var t;let r=e.target;u=r.scrollLeft,d=r.scrollTop,(t=n.onScroll)==null||t.call(n,e)};_(()=>{if(n.nativeScrollbar){let e=r.value;e&&(e.scrollTop=d,e.scrollLeft=u)}});let p={display:`flex`,flexWrap:`nowrap`,width:`100%`,flexDirection:`row`},m={scrollTo:l},h=t(()=>{let{common:{cubicBezierEaseInOut:e},self:t}=c.value;return{"--n-bezier":e,"--n-color":n.embedded?t.colorEmbedded:t.color,"--n-text-color":t.textColor}}),g=o?e(`layout`,t(()=>n.embedded?`e`:``),h,n):void 0;return Object.assign({mergedClsPrefix:a,scrollableElRef:r,scrollbarInstRef:i,hasSiderStyle:p,mergedTheme:c,handleNativeElScroll:f,cssVars:o?void 0:h,themeClass:g?.themeClass,onRender:g?.onRender},m)},render(){var e;let{mergedClsPrefix:t,hasSider:r}=this;(e=this.onRender)==null||e.call(this);let i=r?this.hasSiderStyle:void 0,a=[this.themeClass,n&&`${t}-layout-content`,`${t}-layout`,`${t}-layout--${this.position}-positioned`];return b(`div`,{class:a,style:this.cssVars},this.nativeScrollbar?b(`div`,{ref:`scrollableElRef`,class:[`${t}-layout-scroll-container`,this.contentClass],style:[this.contentStyle,i],onScroll:this.handleNativeElScroll},this.$slots):b(A,Object.assign({},this.scrollbarProps,{onScroll:this.onScroll,ref:`scrollbarInstRef`,theme:this.mergedTheme.peers.Scrollbar,themeOverrides:this.mergedTheme.peerOverrides.Scrollbar,contentClass:this.contentClass,contentStyle:[this.contentStyle,i]}),this.$slots))}})}var K=Te(!1),Ee=d(`layout-header`,`
 transition:
 color .3s var(--n-bezier),
 background-color .3s var(--n-bezier),
 box-shadow .3s var(--n-bezier),
 border-color .3s var(--n-bezier);
 box-sizing: border-box;
 width: 100%;
 background-color: var(--n-color);
 color: var(--n-text-color);
`,[c(`absolute-positioned`,`
 position: absolute;
 left: 0;
 right: 0;
 top: 0;
 `),c(`bordered`,`
 border-bottom: solid 1px var(--n-border-color);
 `)]),De={position:G,inverted:Boolean,bordered:{type:Boolean,default:!1}},Oe=O({name:`LayoutHeader`,props:Object.assign(Object.assign({},F.props),De),setup(n){let{mergedClsPrefixRef:r,inlineThemeDisabled:i}=C(n),a=F(`Layout`,`-layout-header`,Ee,W,n,r),o=t(()=>{let{common:{cubicBezierEaseInOut:e},self:t}=a.value,r={"--n-bezier":e};return n.inverted?(r[`--n-color`]=t.headerColorInverted,r[`--n-text-color`]=t.textColorInverted,r[`--n-border-color`]=t.headerBorderColorInverted):(r[`--n-color`]=t.headerColor,r[`--n-text-color`]=t.textColor,r[`--n-border-color`]=t.headerBorderColor),r}),s=i?e(`layout-header`,t(()=>n.inverted?`a`:`b`),o,n):void 0;return{mergedClsPrefix:r,cssVars:i?void 0:o,themeClass:s?.themeClass,onRender:s?.onRender}},render(){var e;let{mergedClsPrefix:t}=this;return(e=this.onRender)==null||e.call(this),b(`div`,{class:[`${t}-layout-header`,this.themeClass,this.position&&`${t}-layout-header--${this.position}-positioned`,this.bordered&&`${t}-layout-header--bordered`],style:this.cssVars},this.$slots)}}),ke=d(`layout-sider`,`
 flex-shrink: 0;
 box-sizing: border-box;
 position: relative;
 z-index: 1;
 color: var(--n-text-color);
 transition:
 color .3s var(--n-bezier),
 border-color .3s var(--n-bezier),
 min-width .3s var(--n-bezier),
 max-width .3s var(--n-bezier),
 transform .3s var(--n-bezier),
 background-color .3s var(--n-bezier);
 background-color: var(--n-color);
 display: flex;
 justify-content: flex-end;
`,[c(`bordered`,[a(`border`,`
 content: "";
 position: absolute;
 top: 0;
 bottom: 0;
 width: 1px;
 background-color: var(--n-border-color);
 transition: background-color .3s var(--n-bezier);
 `)]),a(`left-placement`,[c(`bordered`,[a(`border`,`
 right: 0;
 `)])]),c(`right-placement`,`
 justify-content: flex-start;
 `,[c(`bordered`,[a(`border`,`
 left: 0;
 `)]),c(`collapsed`,[d(`layout-toggle-button`,[d(`base-icon`,`
 transform: rotate(180deg);
 `)]),d(`layout-toggle-bar`,[u(`&:hover`,[a(`top`,{transform:`rotate(-12deg) scale(1.15) translateY(-2px)`}),a(`bottom`,{transform:`rotate(12deg) scale(1.15) translateY(2px)`})])])]),d(`layout-toggle-button`,`
 left: 0;
 transform: translateX(-50%) translateY(-50%);
 `,[d(`base-icon`,`
 transform: rotate(0);
 `)]),d(`layout-toggle-bar`,`
 left: -28px;
 transform: rotate(180deg);
 `,[u(`&:hover`,[a(`top`,{transform:`rotate(12deg) scale(1.15) translateY(-2px)`}),a(`bottom`,{transform:`rotate(-12deg) scale(1.15) translateY(2px)`})])])]),c(`collapsed`,[d(`layout-toggle-bar`,[u(`&:hover`,[a(`top`,{transform:`rotate(-12deg) scale(1.15) translateY(-2px)`}),a(`bottom`,{transform:`rotate(12deg) scale(1.15) translateY(2px)`})])]),d(`layout-toggle-button`,[d(`base-icon`,`
 transform: rotate(0);
 `)])]),d(`layout-toggle-button`,`
 transition:
 color .3s var(--n-bezier),
 right .3s var(--n-bezier),
 left .3s var(--n-bezier),
 border-color .3s var(--n-bezier),
 background-color .3s var(--n-bezier);
 cursor: pointer;
 width: 24px;
 height: 24px;
 position: absolute;
 top: 50%;
 right: 0;
 border-radius: 50%;
 display: flex;
 align-items: center;
 justify-content: center;
 font-size: 18px;
 color: var(--n-toggle-button-icon-color);
 border: var(--n-toggle-button-border);
 background-color: var(--n-toggle-button-color);
 box-shadow: 0 2px 4px 0px rgba(0, 0, 0, .06);
 transform: translateX(50%) translateY(-50%);
 z-index: 1;
 `,[d(`base-icon`,`
 transition: transform .3s var(--n-bezier);
 transform: rotate(180deg);
 `)]),d(`layout-toggle-bar`,`
 cursor: pointer;
 height: 72px;
 width: 32px;
 position: absolute;
 top: calc(50% - 36px);
 right: -28px;
 `,[a(`top, bottom`,`
 position: absolute;
 width: 4px;
 border-radius: 2px;
 height: 38px;
 left: 14px;
 transition: 
 background-color .3s var(--n-bezier),
 transform .3s var(--n-bezier);
 `),a(`bottom`,`
 position: absolute;
 top: 34px;
 `),u(`&:hover`,[a(`top`,{transform:`rotate(12deg) scale(1.15) translateY(-2px)`}),a(`bottom`,{transform:`rotate(-12deg) scale(1.15) translateY(2px)`})]),a(`top, bottom`,{backgroundColor:`var(--n-toggle-bar-color)`}),u(`&:hover`,[a(`top, bottom`,{backgroundColor:`var(--n-toggle-bar-color-hover)`})])]),a(`border`,`
 position: absolute;
 top: 0;
 right: 0;
 bottom: 0;
 width: 1px;
 transition: background-color .3s var(--n-bezier);
 `),d(`layout-sider-scroll-container`,`
 flex-grow: 1;
 flex-shrink: 0;
 box-sizing: border-box;
 height: 100%;
 opacity: 0;
 transition: opacity .3s var(--n-bezier);
 max-width: 100%;
 `),c(`show-content`,[d(`layout-sider-scroll-container`,{opacity:1})]),c(`absolute-positioned`,`
 position: absolute;
 left: 0;
 top: 0;
 bottom: 0;
 `)]),Ae=O({props:{clsPrefix:{type:String,required:!0},onClick:Function},render(){let{clsPrefix:e}=this;return b(`div`,{onClick:this.onClick,class:`${e}-layout-toggle-bar`},b(`div`,{class:`${e}-layout-toggle-bar__top`}),b(`div`,{class:`${e}-layout-toggle-bar__bottom`}))}}),je=O({name:`LayoutToggleButton`,props:{clsPrefix:{type:String,required:!0},onClick:Function},render(){let{clsPrefix:e}=this;return b(`div`,{class:`${e}-layout-toggle-button`,onClick:this.onClick},b(w,{clsPrefix:e},{default:()=>b(ce,null)}))}}),Me={position:G,bordered:Boolean,collapsedWidth:{type:Number,default:48},width:{type:[Number,String],default:272},contentClass:String,contentStyle:{type:[String,Object],default:``},collapseMode:{type:String,default:`transform`},collapsed:{type:Boolean,default:void 0},defaultCollapsed:Boolean,showCollapsedContent:{type:Boolean,default:!0},showTrigger:{type:[Boolean,String],default:!1},nativeScrollbar:{type:Boolean,default:!0},inverted:Boolean,scrollbarProps:Object,triggerClass:String,triggerStyle:[String,Object],collapsedTriggerClass:String,collapsedTriggerStyle:[String,Object],"onUpdate:collapsed":[Function,Array],onUpdateCollapsed:[Function,Array],onAfterEnter:Function,onAfterLeave:Function,onExpand:[Function,Array],onCollapse:[Function,Array],onScroll:Function},Ne=O({name:`LayoutSider`,props:Object.assign(Object.assign({},F.props),Me),setup(r){let i=N(we),a=s(null),o=s(null),c=s(r.defaultCollapsed),l=H(n(r,`collapsed`),c),u=t(()=>U(l.value?r.collapsedWidth:r.width)),d=t(()=>r.collapseMode===`transform`?{minWidth:U(r.width)}:{}),f=t(()=>i?i.siderPlacement:`left`);function p(e,t){if(r.nativeScrollbar){let{value:n}=a;n&&(t===void 0?n.scrollTo(e):n.scrollTo(e,t))}else{let{value:n}=o;n&&n.scrollTo(e,t)}}function m(){let{"onUpdate:collapsed":e,onUpdateCollapsed:t,onExpand:n,onCollapse:i}=r,{value:a}=l;t&&P(t,!a),e&&P(e,!a),c.value=!a,a?n&&P(n):i&&P(i)}let h=0,g=0,v=e=>{var t;let n=e.target;h=n.scrollLeft,g=n.scrollTop,(t=r.onScroll)==null||t.call(r,e)};_(()=>{if(r.nativeScrollbar){let e=a.value;e&&(e.scrollTop=g,e.scrollLeft=h)}}),y(xe,{collapsedRef:l,collapseModeRef:n(r,`collapseMode`)});let{mergedClsPrefixRef:b,inlineThemeDisabled:x}=C(r),S=F(`Layout`,`-layout-sider`,ke,W,r,b);function w(e){var t,n;e.propertyName===`max-width`&&(l.value?(t=r.onAfterLeave)==null||t.call(r):(n=r.onAfterEnter)==null||n.call(r))}let T={scrollTo:p},E=t(()=>{let{common:{cubicBezierEaseInOut:e},self:t}=S.value,{siderToggleButtonColor:n,siderToggleButtonBorder:i,siderToggleBarColor:a,siderToggleBarColorHover:o}=t,s={"--n-bezier":e,"--n-toggle-button-color":n,"--n-toggle-button-border":i,"--n-toggle-bar-color":a,"--n-toggle-bar-color-hover":o};return r.inverted?(s[`--n-color`]=t.siderColorInverted,s[`--n-text-color`]=t.textColorInverted,s[`--n-border-color`]=t.siderBorderColorInverted,s[`--n-toggle-button-icon-color`]=t.siderToggleButtonIconColorInverted,s.__invertScrollbar=t.__invertScrollbar):(s[`--n-color`]=t.siderColor,s[`--n-text-color`]=t.textColor,s[`--n-border-color`]=t.siderBorderColor,s[`--n-toggle-button-icon-color`]=t.siderToggleButtonIconColor),s}),D=x?e(`layout-sider`,t(()=>r.inverted?`a`:`b`),E,r):void 0;return Object.assign({scrollableElRef:a,scrollbarInstRef:o,mergedClsPrefix:b,mergedTheme:S,styleMaxWidth:u,mergedCollapsed:l,scrollContainerStyle:d,siderPlacement:f,handleNativeElScroll:v,handleTransitionend:w,handleTriggerClick:m,inlineThemeDisabled:x,cssVars:E,themeClass:D?.themeClass,onRender:D?.onRender},T)},render(){var e;let{mergedClsPrefix:t,mergedCollapsed:n,showTrigger:r}=this;return(e=this.onRender)==null||e.call(this),b(`aside`,{class:[`${t}-layout-sider`,this.themeClass,`${t}-layout-sider--${this.position}-positioned`,`${t}-layout-sider--${this.siderPlacement}-placement`,this.bordered&&`${t}-layout-sider--bordered`,n&&`${t}-layout-sider--collapsed`,(!n||this.showCollapsedContent)&&`${t}-layout-sider--show-content`],onTransitionend:this.handleTransitionend,style:[this.inlineThemeDisabled?void 0:this.cssVars,{maxWidth:this.styleMaxWidth,width:U(this.width)}]},this.nativeScrollbar?b(`div`,{class:[`${t}-layout-sider-scroll-container`,this.contentClass],onScroll:this.handleNativeElScroll,style:[this.scrollContainerStyle,{overflow:`auto`},this.contentStyle],ref:`scrollableElRef`},this.$slots):b(A,Object.assign({},this.scrollbarProps,{onScroll:this.onScroll,ref:`scrollbarInstRef`,style:this.scrollContainerStyle,contentStyle:this.contentStyle,contentClass:this.contentClass,theme:this.mergedTheme.peers.Scrollbar,themeOverrides:this.mergedTheme.peerOverrides.Scrollbar,builtinThemeOverrides:this.inverted&&this.cssVars.__invertScrollbar===`true`?{colorHover:`rgba(255, 255, 255, .4)`,color:`rgba(255, 255, 255, .3)`}:void 0}),this.$slots),r?b(r===`bar`?Ae:je,{clsPrefix:t,class:n?this.collapsedTriggerClass:this.triggerClass,style:n?this.collapsedTriggerStyle:this.triggerStyle,onClick:this.handleTriggerClick}):null,this.bordered?b(`div`,{class:`${t}-layout-sider__border`}):null)}}),q=R(`n-menu`),Pe=R(`n-submenu`),J=R(`n-menu-item-group`),Fe=[u(`&::before`,`background-color: var(--n-item-color-hover);`),a(`arrow`,`
 color: var(--n-arrow-color-hover);
 `),a(`icon`,`
 color: var(--n-item-icon-color-hover);
 `),d(`menu-item-content-header`,`
 color: var(--n-item-text-color-hover);
 `,[u(`a`,`
 color: var(--n-item-text-color-hover);
 `),a(`extra`,`
 color: var(--n-item-text-color-hover);
 `)])],Ie=[a(`icon`,`
 color: var(--n-item-icon-color-hover-horizontal);
 `),d(`menu-item-content-header`,`
 color: var(--n-item-text-color-hover-horizontal);
 `,[u(`a`,`
 color: var(--n-item-text-color-hover-horizontal);
 `),a(`extra`,`
 color: var(--n-item-text-color-hover-horizontal);
 `)])],Le=u([d(`menu`,`
 background-color: var(--n-color);
 color: var(--n-item-text-color);
 overflow: hidden;
 transition: background-color .3s var(--n-bezier);
 box-sizing: border-box;
 font-size: var(--n-font-size);
 padding-bottom: 6px;
 `,[c(`horizontal`,`
 max-width: 100%;
 width: 100%;
 display: flex;
 overflow: hidden;
 padding-bottom: 0;
 `,[d(`submenu`,`margin: 0;`),d(`menu-item`,`margin: 0;`),d(`menu-item-content`,`
 padding: 0 20px;
 border-bottom: 2px solid #0000;
 `,[u(`&::before`,`display: none;`),c(`selected`,`border-bottom: 2px solid var(--n-border-color-horizontal)`)]),d(`menu-item-content`,[c(`selected`,[a(`icon`,`color: var(--n-item-icon-color-active-horizontal);`),d(`menu-item-content-header`,`
 color: var(--n-item-text-color-active-horizontal);
 `,[u(`a`,`color: var(--n-item-text-color-active-horizontal);`),a(`extra`,`color: var(--n-item-text-color-active-horizontal);`)])]),c(`child-active`,`
 border-bottom: 2px solid var(--n-border-color-horizontal);
 `,[d(`menu-item-content-header`,`
 color: var(--n-item-text-color-child-active-horizontal);
 `,[u(`a`,`
 color: var(--n-item-text-color-child-active-horizontal);
 `),a(`extra`,`
 color: var(--n-item-text-color-child-active-horizontal);
 `)]),a(`icon`,`
 color: var(--n-item-icon-color-child-active-horizontal);
 `)]),l(`disabled`,[l(`selected, child-active`,[u(`&:focus-within`,Ie)]),c(`selected`,[Y(null,[a(`icon`,`color: var(--n-item-icon-color-active-hover-horizontal);`),d(`menu-item-content-header`,`
 color: var(--n-item-text-color-active-hover-horizontal);
 `,[u(`a`,`color: var(--n-item-text-color-active-hover-horizontal);`),a(`extra`,`color: var(--n-item-text-color-active-hover-horizontal);`)])])]),c(`child-active`,[Y(null,[a(`icon`,`color: var(--n-item-icon-color-child-active-hover-horizontal);`),d(`menu-item-content-header`,`
 color: var(--n-item-text-color-child-active-hover-horizontal);
 `,[u(`a`,`color: var(--n-item-text-color-child-active-hover-horizontal);`),a(`extra`,`color: var(--n-item-text-color-child-active-hover-horizontal);`)])])]),Y(`border-bottom: 2px solid var(--n-border-color-horizontal);`,Ie)]),d(`menu-item-content-header`,[u(`a`,`color: var(--n-item-text-color-horizontal);`)])])]),l(`responsive`,[d(`menu-item-content-header`,`
 overflow: hidden;
 text-overflow: ellipsis;
 `)]),c(`collapsed`,[d(`menu-item-content`,[c(`selected`,[u(`&::before`,`
 background-color: var(--n-item-color-active-collapsed) !important;
 `)]),d(`menu-item-content-header`,`opacity: 0;`),a(`arrow`,`opacity: 0;`),a(`icon`,`color: var(--n-item-icon-color-collapsed);`)])]),d(`menu-item`,`
 height: var(--n-item-height);
 margin-top: 6px;
 position: relative;
 `),d(`menu-item-content`,`
 box-sizing: border-box;
 line-height: 1.75;
 height: 100%;
 display: grid;
 grid-template-areas: "icon content arrow";
 grid-template-columns: auto 1fr auto;
 align-items: center;
 cursor: pointer;
 position: relative;
 padding-right: 18px;
 transition:
 background-color .3s var(--n-bezier),
 padding-left .3s var(--n-bezier),
 border-color .3s var(--n-bezier);
 `,[u(`> *`,`z-index: 1;`),u(`&::before`,`
 z-index: auto;
 content: "";
 background-color: #0000;
 position: absolute;
 left: 8px;
 right: 8px;
 top: 0;
 bottom: 0;
 pointer-events: none;
 border-radius: var(--n-border-radius);
 transition: background-color .3s var(--n-bezier);
 `),c(`disabled`,`
 opacity: .45;
 cursor: not-allowed;
 `),c(`collapsed`,[a(`arrow`,`transform: rotate(0);`)]),c(`selected`,[u(`&::before`,`background-color: var(--n-item-color-active);`),a(`arrow`,`color: var(--n-arrow-color-active);`),a(`icon`,`color: var(--n-item-icon-color-active);`),d(`menu-item-content-header`,`
 color: var(--n-item-text-color-active);
 `,[u(`a`,`color: var(--n-item-text-color-active);`),a(`extra`,`color: var(--n-item-text-color-active);`)])]),c(`child-active`,[d(`menu-item-content-header`,`
 color: var(--n-item-text-color-child-active);
 `,[u(`a`,`
 color: var(--n-item-text-color-child-active);
 `),a(`extra`,`
 color: var(--n-item-text-color-child-active);
 `)]),a(`arrow`,`
 color: var(--n-arrow-color-child-active);
 `),a(`icon`,`
 color: var(--n-item-icon-color-child-active);
 `)]),l(`disabled`,[l(`selected, child-active`,[u(`&:focus-within`,Fe)]),c(`selected`,[Y(null,[a(`arrow`,`color: var(--n-arrow-color-active-hover);`),a(`icon`,`color: var(--n-item-icon-color-active-hover);`),d(`menu-item-content-header`,`
 color: var(--n-item-text-color-active-hover);
 `,[u(`a`,`color: var(--n-item-text-color-active-hover);`),a(`extra`,`color: var(--n-item-text-color-active-hover);`)])])]),c(`child-active`,[Y(null,[a(`arrow`,`color: var(--n-arrow-color-child-active-hover);`),a(`icon`,`color: var(--n-item-icon-color-child-active-hover);`),d(`menu-item-content-header`,`
 color: var(--n-item-text-color-child-active-hover);
 `,[u(`a`,`color: var(--n-item-text-color-child-active-hover);`),a(`extra`,`color: var(--n-item-text-color-child-active-hover);`)])])]),c(`selected`,[Y(null,[u(`&::before`,`background-color: var(--n-item-color-active-hover);`)])]),Y(null,Fe)]),a(`icon`,`
 grid-area: icon;
 color: var(--n-item-icon-color);
 transition:
 color .3s var(--n-bezier),
 font-size .3s var(--n-bezier),
 margin-right .3s var(--n-bezier);
 box-sizing: content-box;
 display: inline-flex;
 align-items: center;
 justify-content: center;
 `),a(`arrow`,`
 grid-area: arrow;
 font-size: 16px;
 color: var(--n-arrow-color);
 transform: rotate(180deg);
 opacity: 1;
 transition:
 color .3s var(--n-bezier),
 transform 0.2s var(--n-bezier),
 opacity 0.2s var(--n-bezier);
 `),d(`menu-item-content-header`,`
 grid-area: content;
 transition:
 color .3s var(--n-bezier),
 opacity .3s var(--n-bezier);
 opacity: 1;
 white-space: nowrap;
 color: var(--n-item-text-color);
 `,[u(`a`,`
 outline: none;
 text-decoration: none;
 transition: color .3s var(--n-bezier);
 color: var(--n-item-text-color);
 `,[u(`&::before`,`
 content: "";
 position: absolute;
 left: 0;
 right: 0;
 top: 0;
 bottom: 0;
 `)]),a(`extra`,`
 font-size: .93em;
 color: var(--n-group-text-color);
 transition: color .3s var(--n-bezier);
 `)])]),d(`submenu`,`
 cursor: pointer;
 position: relative;
 margin-top: 6px;
 `,[d(`menu-item-content`,`
 height: var(--n-item-height);
 `),d(`submenu-children`,`
 overflow: hidden;
 padding: 0;
 `,[me({duration:`.2s`})])]),d(`menu-item-group`,[d(`menu-item-group-title`,`
 margin-top: 6px;
 color: var(--n-group-text-color);
 cursor: default;
 font-size: .93em;
 height: 36px;
 display: flex;
 align-items: center;
 transition:
 padding-left .3s var(--n-bezier),
 color .3s var(--n-bezier);
 `)])]),d(`menu-tooltip`,[u(`a`,`
 color: inherit;
 text-decoration: none;
 `)]),d(`menu-divider`,`
 transition: background-color .3s var(--n-bezier);
 background-color: var(--n-divider-color);
 height: 1px;
 margin: 6px 18px;
 `)]);function Y(e,t){return[c(`hover`,e,t),u(`&:hover`,e,t)]}var Re=O({name:`MenuOptionContent`,props:{collapsed:Boolean,disabled:Boolean,title:[String,Function],icon:Function,extra:[String,Function],showArrow:Boolean,childActive:Boolean,hover:Boolean,paddingLeft:Number,selected:Boolean,maxIconSize:{type:Number,required:!0},activeIconSize:{type:Number,required:!0},iconMarginRight:{type:Number,required:!0},clsPrefix:{type:String,required:!0},onClick:Function,tmNode:{type:Object,required:!0},isEllipsisPlaceholder:Boolean},setup(e){let{props:n}=N(q);return{menuProps:n,style:t(()=>{let{paddingLeft:t}=e;return{paddingLeft:t&&`${t}px`}}),iconStyle:t(()=>{let{maxIconSize:t,activeIconSize:n,iconMarginRight:r}=e;return{width:`${t}px`,height:`${t}px`,fontSize:`${n}px`,marginRight:`${r}px`}})}},render(){let{clsPrefix:e,tmNode:t,menuProps:{renderIcon:n,renderLabel:r,renderExtra:i,expandIcon:a}}=this,o=n?n(t.rawNode):V(this.icon);return b(`div`,{onClick:e=>{var t;(t=this.onClick)==null||t.call(this,e)},role:`none`,class:[`${e}-menu-item-content`,{[`${e}-menu-item-content--selected`]:this.selected,[`${e}-menu-item-content--collapsed`]:this.collapsed,[`${e}-menu-item-content--child-active`]:this.childActive,[`${e}-menu-item-content--disabled`]:this.disabled,[`${e}-menu-item-content--hover`]:this.hover}],style:this.style},o&&b(`div`,{class:`${e}-menu-item-content__icon`,style:this.iconStyle,role:`none`},[o]),b(`div`,{class:`${e}-menu-item-content-header`,role:`none`},this.isEllipsisPlaceholder?this.title:r?r(t.rawNode):V(this.title),this.extra||i?b(`span`,{class:`${e}-menu-item-content-header__extra`},` `,i?i(t.rawNode):V(this.extra)):null),this.showArrow?b(w,{ariaHidden:!0,class:`${e}-menu-item-content__arrow`,clsPrefix:e},{default:()=>a?a(t.rawNode):b(ge,null)}):null)}}),ze=8;function X(e){let n=N(q),{props:r,mergedCollapsedRef:i}=n,a=N(Pe,null),o=N(J,null),s=t(()=>r.mode===`horizontal`),c=t(()=>s.value?r.dropdownPlacement:`tmNodes`in e?`right-start`:`right`),l=t(()=>Math.max(r.collapsedIconSize??r.iconSize,r.iconSize));return{dropdownPlacement:c,activeIconSize:t(()=>!s.value&&e.root&&i.value?r.collapsedIconSize??r.iconSize:r.iconSize),maxIconSize:l,paddingLeft:t(()=>{if(s.value)return;let{collapsedWidth:t,indent:n,rootIndent:c}=r,{root:u,isGroup:d}=e,f=c===void 0?n:c;return u?i.value?t/2-l.value/2:f:o&&typeof o.paddingLeftRef.value==`number`?n/2+o.paddingLeftRef.value:a&&typeof a.paddingLeftRef.value==`number`?(d?n/2:n)+a.paddingLeftRef.value:0}),iconMarginRight:t(()=>{let{collapsedWidth:t,indent:n,rootIndent:a}=r,{value:o}=l,{root:c}=e;return s.value||!c||!i.value?ze:(a===void 0?n:a)+o+ze-(t+o)/2}),NMenu:n,NSubmenu:a,NMenuOptionGroup:o}}var Z={internalKey:{type:[String,Number],required:!0},root:Boolean,isGroup:Boolean,level:{type:Number,required:!0},title:[String,Function],extra:[String,Function]},Be=O({name:`MenuDivider`,setup(){let{mergedClsPrefixRef:e,isHorizontalRef:t}=N(q);return()=>t.value?null:b(`div`,{class:`${e.value}-menu-divider`})}}),Ve=Object.assign(Object.assign({},Z),{tmNode:{type:Object,required:!0},disabled:Boolean,icon:Function,onClick:Function}),He=D(Ve),Ue=O({name:`MenuOption`,props:Ve,setup(e){let n=X(e),{NSubmenu:r,NMenu:i,NMenuOptionGroup:a}=n,{props:o,mergedClsPrefixRef:s,mergedCollapsedRef:c}=i,l=r?r.mergedDisabledRef:a?a.mergedDisabledRef:{value:!1},u=t(()=>l.value||e.disabled);function d(t){let{onClick:n}=e;n&&n(t)}function f(t){u.value||(i.doSelect(e.internalKey,e.tmNode.rawNode),d(t))}return{mergedClsPrefix:s,dropdownPlacement:n.dropdownPlacement,paddingLeft:n.paddingLeft,iconMarginRight:n.iconMarginRight,maxIconSize:n.maxIconSize,activeIconSize:n.activeIconSize,mergedTheme:i.mergedThemeRef,menuProps:o,dropdownEnabled:L(()=>e.root&&c.value&&o.mode!==`horizontal`&&!u.value),selected:L(()=>i.mergedValueRef.value===e.internalKey),mergedDisabled:u,handleClick:f}},render(){let{mergedClsPrefix:e,mergedTheme:t,tmNode:n,menuProps:{renderLabel:r,nodeProps:i}}=this,a=i?.(n.rawNode);return b(`div`,Object.assign({},a,{role:`menuitem`,class:[`${e}-menu-item`,a?.class]}),b(ae,{theme:t.peers.Tooltip,themeOverrides:t.peerOverrides.Tooltip,trigger:`hover`,placement:this.dropdownPlacement,disabled:!this.dropdownEnabled||this.title===void 0,internalExtraClass:[`menu-tooltip`]},{default:()=>r?r(n.rawNode):V(this.title),trigger:()=>b(Re,{tmNode:n,clsPrefix:e,paddingLeft:this.paddingLeft,iconMarginRight:this.iconMarginRight,maxIconSize:this.maxIconSize,activeIconSize:this.activeIconSize,selected:this.selected,title:this.title,extra:this.extra,disabled:this.mergedDisabled,icon:this.icon,onClick:this.handleClick})}))}}),We=Object.assign(Object.assign({},Z),{tmNode:{type:Object,required:!0},tmNodes:{type:Array,required:!0}}),Ge=D(We),Ke=O({name:`MenuOptionGroup`,props:We,setup(e){let n=X(e),{NSubmenu:r}=n,i=t(()=>r?.mergedDisabledRef.value?!0:e.tmNode.disabled);y(J,{paddingLeftRef:n.paddingLeft,mergedDisabledRef:i});let{mergedClsPrefixRef:a,props:o}=N(q);return function(){let{value:t}=a,r=n.paddingLeft.value,{nodeProps:i}=o,s=i?.(e.tmNode.rawNode);return b(`div`,{class:`${t}-menu-item-group`,role:`group`},b(`div`,Object.assign({},s,{class:[`${t}-menu-item-group-title`,s?.class],style:[s?.style||``,r===void 0?``:`padding-left: ${r}px;`]}),V(e.title),e.extra?b(h,null,` `,V(e.extra)):null),b(`div`,null,e.tmNodes.map(e=>$(e,o))))}}});function Q(e){return e.type===`divider`||e.type===`render`}function qe(e){return e.type===`divider`}function $(e,t){let{rawNode:n}=e,{show:r}=n;if(r===!1)return null;if(Q(n))return qe(n)?b(Be,Object.assign({key:e.key},n.props)):null;let{labelField:i}=t,{key:a,level:o,isGroup:s}=e,c=Object.assign(Object.assign({},n),{title:n.title||n[i],extra:n.titleExtra||n.extra,key:a,internalKey:a,level:o,root:o===0,isGroup:s});return e.children?e.isGroup?b(Ke,B(c,Ge,{tmNode:e,tmNodes:e.children,key:a})):b(Xe,B(c,Ye,{key:a,rawNodes:n[t.childrenField],tmNodes:e.children,tmNode:e})):b(Ue,B(c,He,{key:a,tmNode:e}))}var Je=Object.assign(Object.assign({},Z),{rawNodes:{type:Array,default:()=>[]},tmNodes:{type:Array,default:()=>[]},tmNode:{type:Object,required:!0},disabled:Boolean,icon:Function,onClick:Function,domId:String,virtualChildActive:{type:Boolean,default:void 0},isEllipsisPlaceholder:Boolean}),Ye=D(Je),Xe=O({name:`Submenu`,props:Je,setup(e){let n=X(e),{NMenu:r,NSubmenu:i}=n,{props:a,mergedCollapsedRef:o,mergedThemeRef:c}=r,l=t(()=>{let{disabled:t}=e;return i?.mergedDisabledRef.value||a.disabled?!0:t}),u=s(!1);y(Pe,{paddingLeftRef:n.paddingLeft,mergedDisabledRef:l}),y(J,null);function d(){let{onClick:t}=e;t&&t()}function f(){l.value||(o.value||r.toggleExpand(e.internalKey),d())}function p(e){u.value=e}return{menuProps:a,mergedTheme:c,doSelect:r.doSelect,inverted:r.invertedRef,isHorizontal:r.isHorizontalRef,mergedClsPrefix:r.mergedClsPrefixRef,maxIconSize:n.maxIconSize,activeIconSize:n.activeIconSize,iconMarginRight:n.iconMarginRight,dropdownPlacement:n.dropdownPlacement,dropdownShow:u,paddingLeft:n.paddingLeft,mergedDisabled:l,mergedValue:r.mergedValueRef,childActive:L(()=>e.virtualChildActive??r.activePathRef.value.includes(e.internalKey)),collapsed:t(()=>a.mode===`horizontal`?!1:o.value?!0:!r.mergedExpandedKeysRef.value.includes(e.internalKey)),dropdownEnabled:t(()=>!l.value&&(a.mode===`horizontal`||o.value)),handlePopoverShowChange:p,handleClick:f}},render(){let{mergedClsPrefix:e,menuProps:{renderIcon:t,renderLabel:n}}=this,r=()=>{let{isHorizontal:e,paddingLeft:t,collapsed:n,mergedDisabled:r,maxIconSize:i,activeIconSize:a,title:o,childActive:s,icon:c,handleClick:l,menuProps:{nodeProps:u},dropdownShow:d,iconMarginRight:f,tmNode:p,mergedClsPrefix:m,isEllipsisPlaceholder:h,extra:g}=this,_=u?.(p.rawNode);return b(`div`,Object.assign({},_,{class:[`${m}-menu-item`,_?.class],role:`menuitem`}),b(Re,{tmNode:p,paddingLeft:t,collapsed:n,disabled:r,iconMarginRight:f,maxIconSize:i,activeIconSize:a,title:o,extra:g,showArrow:!e,childActive:s,clsPrefix:m,icon:c,hover:d,onClick:l,isEllipsisPlaceholder:h}))},i=()=>b(te,null,{default:()=>{let{tmNodes:t,collapsed:n}=this;return n?null:b(`div`,{class:`${e}-submenu-children`,role:`menu`},t.map(e=>$(e,this.menuProps)))}});return this.root?b(se,Object.assign({size:`large`,trigger:`hover`},this.menuProps?.dropdownProps,{themeOverrides:this.mergedTheme.peerOverrides.Dropdown,theme:this.mergedTheme.peers.Dropdown,builtinThemeOverrides:{fontSizeLarge:`14px`,optionIconSizeLarge:`18px`},value:this.mergedValue,disabled:!this.dropdownEnabled,placement:this.dropdownPlacement,keyField:this.menuProps.keyField,labelField:this.menuProps.labelField,childrenField:this.menuProps.childrenField,onUpdateShow:this.handlePopoverShowChange,options:this.rawNodes,onSelect:this.doSelect,inverted:this.inverted,renderIcon:t,renderLabel:n}),{default:()=>b(`div`,{class:`${e}-submenu`,role:`menu`,"aria-expanded":!this.collapsed,id:this.domId},r(),this.isHorizontal?null:i())}):b(`div`,{class:`${e}-submenu`,role:`menu`,"aria-expanded":!this.collapsed,id:this.domId},r(),i())}}),Ze=Object.assign(Object.assign({},F.props),{options:{type:Array,default:()=>[]},collapsed:{type:Boolean,default:void 0},collapsedWidth:{type:Number,default:48},iconSize:{type:Number,default:20},collapsedIconSize:{type:Number,default:24},rootIndent:Number,indent:{type:Number,default:32},labelField:{type:String,default:`label`},keyField:{type:String,default:`key`},childrenField:{type:String,default:`children`},disabledField:{type:String,default:`disabled`},defaultExpandAll:Boolean,defaultExpandedKeys:Array,expandedKeys:Array,value:[String,Number],defaultValue:{type:[String,Number],default:null},mode:{type:String,default:`vertical`},watchProps:{type:Array,default:void 0},disabled:Boolean,show:{type:Boolean,default:!0},inverted:Boolean,"onUpdate:expandedKeys":[Function,Array],onUpdateExpandedKeys:[Function,Array],onUpdateValue:[Function,Array],"onUpdate:value":[Function,Array],expandIcon:Function,renderIcon:Function,renderLabel:Function,renderExtra:Function,dropdownProps:Object,accordion:Boolean,nodeProps:Function,dropdownPlacement:{type:String,default:`bottom`},responsive:Boolean,items:Array,onOpenNamesChange:[Function,Array],onSelect:[Function,Array],onExpandedNamesChange:[Function,Array],expandedNames:Array,defaultExpandedNames:Array}),Qe=O({name:`Menu`,inheritAttrs:!1,props:Ze,setup(r){let{mergedClsPrefixRef:i,inlineThemeDisabled:a}=C(r),o=F(`Menu`,`-menu`,Le,be,r,i),c=N(xe,null),l=t(()=>{let{collapsed:e}=r;if(e!==void 0)return e;if(c){let{collapseModeRef:e,collapsedRef:t}=c;if(e.value===`width`)return t.value??!1}return!1}),u=t(()=>{let{keyField:e,childrenField:t,disabledField:n}=r;return z(r.items||r.options,{getIgnored(e){return Q(e)},getChildren(e){return e[t]},getDisabled(e){return e[n]},getKey(t){return t[e]??t.name}})}),d=t(()=>new Set(u.value.treeNodes.map(e=>e.key))),{watchProps:p}=r,m=s(null);p?.includes(`defaultValue`)?f(()=>{m.value=r.defaultValue}):m.value=r.defaultValue;let h=n(r,`value`),g=H(h,m),_=s([]),v=()=>{_.value=r.defaultExpandAll?u.value.getNonLeafKeys():r.defaultExpandedNames||r.defaultExpandedKeys||u.value.getPath(g.value,{includeSelf:!1}).keyPath};p?.includes(`defaultExpandedKeys`)?f(v):v();let x=de(r,[`expandedNames`,`expandedKeys`]),S=H(x,_),w=t(()=>u.value.treeNodes),T=t(()=>u.value.getPath(g.value).keyPath);y(q,{props:r,mergedCollapsedRef:l,mergedThemeRef:o,mergedValueRef:g,mergedExpandedKeysRef:S,activePathRef:T,mergedClsPrefixRef:i,isHorizontalRef:t(()=>r.mode===`horizontal`),invertedRef:n(r,`inverted`),doSelect:E,toggleExpand:ee});function E(e,t){let{"onUpdate:value":n,onUpdateValue:i,onSelect:a}=r;i&&P(i,e,t),n&&P(n,e,t),a&&P(a,e,t),m.value=e}function D(e){let{"onUpdate:expandedKeys":t,onUpdateExpandedKeys:n,onExpandedNamesChange:i,onOpenNamesChange:a}=r;t&&P(t,e),n&&P(n,e),i&&P(i,e),a&&P(a,e),_.value=e}function ee(e){let t=Array.from(S.value),n=t.findIndex(t=>t===e);if(~n)t.splice(n,1);else{if(r.accordion&&d.value.has(e)){let e=t.findIndex(e=>d.value.has(e));e>-1&&t.splice(e,1)}t.push(e)}D(t)}let O=e=>{let t=u.value.getPath(e??g.value,{includeSelf:!1}).keyPath;if(!t.length)return;let n=Array.from(S.value),i=new Set([...n,...t]);r.accordion&&d.value.forEach(e=>{i.has(e)&&!t.includes(e)&&i.delete(e)}),D(Array.from(i))},te=t(()=>{let{inverted:e}=r,{common:{cubicBezierEaseInOut:t},self:n}=o.value,{borderRadius:i,borderColorHorizontal:a,fontSize:s,itemHeight:c,dividerColor:l}=n,u={"--n-divider-color":l,"--n-bezier":t,"--n-font-size":s,"--n-border-color-horizontal":a,"--n-border-radius":i,"--n-item-height":c};return e?(u[`--n-group-text-color`]=n.groupTextColorInverted,u[`--n-color`]=n.colorInverted,u[`--n-item-text-color`]=n.itemTextColorInverted,u[`--n-item-text-color-hover`]=n.itemTextColorHoverInverted,u[`--n-item-text-color-active`]=n.itemTextColorActiveInverted,u[`--n-item-text-color-child-active`]=n.itemTextColorChildActiveInverted,u[`--n-item-text-color-child-active-hover`]=n.itemTextColorChildActiveInverted,u[`--n-item-text-color-active-hover`]=n.itemTextColorActiveHoverInverted,u[`--n-item-icon-color`]=n.itemIconColorInverted,u[`--n-item-icon-color-hover`]=n.itemIconColorHoverInverted,u[`--n-item-icon-color-active`]=n.itemIconColorActiveInverted,u[`--n-item-icon-color-active-hover`]=n.itemIconColorActiveHoverInverted,u[`--n-item-icon-color-child-active`]=n.itemIconColorChildActiveInverted,u[`--n-item-icon-color-child-active-hover`]=n.itemIconColorChildActiveHoverInverted,u[`--n-item-icon-color-collapsed`]=n.itemIconColorCollapsedInverted,u[`--n-item-text-color-horizontal`]=n.itemTextColorHorizontalInverted,u[`--n-item-text-color-hover-horizontal`]=n.itemTextColorHoverHorizontalInverted,u[`--n-item-text-color-active-horizontal`]=n.itemTextColorActiveHorizontalInverted,u[`--n-item-text-color-child-active-horizontal`]=n.itemTextColorChildActiveHorizontalInverted,u[`--n-item-text-color-child-active-hover-horizontal`]=n.itemTextColorChildActiveHoverHorizontalInverted,u[`--n-item-text-color-active-hover-horizontal`]=n.itemTextColorActiveHoverHorizontalInverted,u[`--n-item-icon-color-horizontal`]=n.itemIconColorHorizontalInverted,u[`--n-item-icon-color-hover-horizontal`]=n.itemIconColorHoverHorizontalInverted,u[`--n-item-icon-color-active-horizontal`]=n.itemIconColorActiveHorizontalInverted,u[`--n-item-icon-color-active-hover-horizontal`]=n.itemIconColorActiveHoverHorizontalInverted,u[`--n-item-icon-color-child-active-horizontal`]=n.itemIconColorChildActiveHorizontalInverted,u[`--n-item-icon-color-child-active-hover-horizontal`]=n.itemIconColorChildActiveHoverHorizontalInverted,u[`--n-arrow-color`]=n.arrowColorInverted,u[`--n-arrow-color-hover`]=n.arrowColorHoverInverted,u[`--n-arrow-color-active`]=n.arrowColorActiveInverted,u[`--n-arrow-color-active-hover`]=n.arrowColorActiveHoverInverted,u[`--n-arrow-color-child-active`]=n.arrowColorChildActiveInverted,u[`--n-arrow-color-child-active-hover`]=n.arrowColorChildActiveHoverInverted,u[`--n-item-color-hover`]=n.itemColorHoverInverted,u[`--n-item-color-active`]=n.itemColorActiveInverted,u[`--n-item-color-active-hover`]=n.itemColorActiveHoverInverted,u[`--n-item-color-active-collapsed`]=n.itemColorActiveCollapsedInverted):(u[`--n-group-text-color`]=n.groupTextColor,u[`--n-color`]=n.color,u[`--n-item-text-color`]=n.itemTextColor,u[`--n-item-text-color-hover`]=n.itemTextColorHover,u[`--n-item-text-color-active`]=n.itemTextColorActive,u[`--n-item-text-color-child-active`]=n.itemTextColorChildActive,u[`--n-item-text-color-child-active-hover`]=n.itemTextColorChildActiveHover,u[`--n-item-text-color-active-hover`]=n.itemTextColorActiveHover,u[`--n-item-icon-color`]=n.itemIconColor,u[`--n-item-icon-color-hover`]=n.itemIconColorHover,u[`--n-item-icon-color-active`]=n.itemIconColorActive,u[`--n-item-icon-color-active-hover`]=n.itemIconColorActiveHover,u[`--n-item-icon-color-child-active`]=n.itemIconColorChildActive,u[`--n-item-icon-color-child-active-hover`]=n.itemIconColorChildActiveHover,u[`--n-item-icon-color-collapsed`]=n.itemIconColorCollapsed,u[`--n-item-text-color-horizontal`]=n.itemTextColorHorizontal,u[`--n-item-text-color-hover-horizontal`]=n.itemTextColorHoverHorizontal,u[`--n-item-text-color-active-horizontal`]=n.itemTextColorActiveHorizontal,u[`--n-item-text-color-child-active-horizontal`]=n.itemTextColorChildActiveHorizontal,u[`--n-item-text-color-child-active-hover-horizontal`]=n.itemTextColorChildActiveHoverHorizontal,u[`--n-item-text-color-active-hover-horizontal`]=n.itemTextColorActiveHoverHorizontal,u[`--n-item-icon-color-horizontal`]=n.itemIconColorHorizontal,u[`--n-item-icon-color-hover-horizontal`]=n.itemIconColorHoverHorizontal,u[`--n-item-icon-color-active-horizontal`]=n.itemIconColorActiveHorizontal,u[`--n-item-icon-color-active-hover-horizontal`]=n.itemIconColorActiveHoverHorizontal,u[`--n-item-icon-color-child-active-horizontal`]=n.itemIconColorChildActiveHorizontal,u[`--n-item-icon-color-child-active-hover-horizontal`]=n.itemIconColorChildActiveHoverHorizontal,u[`--n-arrow-color`]=n.arrowColor,u[`--n-arrow-color-hover`]=n.arrowColorHover,u[`--n-arrow-color-active`]=n.arrowColorActive,u[`--n-arrow-color-active-hover`]=n.arrowColorActiveHover,u[`--n-arrow-color-child-active`]=n.arrowColorChildActive,u[`--n-arrow-color-child-active-hover`]=n.arrowColorChildActiveHover,u[`--n-item-color-hover`]=n.itemColorHover,u[`--n-item-color-active`]=n.itemColorActive,u[`--n-item-color-active-hover`]=n.itemColorActiveHover,u[`--n-item-color-active-collapsed`]=n.itemColorActiveCollapsed),u}),k=a?e(`menu`,t(()=>r.inverted?`a`:`b`),te,r):void 0,A=le(),j=s(null),M=s(null),I=!0,L=()=>{var e;I?I=!1:(e=j.value)==null||e.sync({showAllItemsBeforeCalculate:!0})};function ne(){return document.getElementById(A)}let R=s(-1);function re(e){R.value=r.options.length-e}function ie(e){e||(R.value=-1)}let ae=t(()=>{let e=R.value;return{children:e===-1?[]:r.options.slice(e)}}),oe=t(()=>{let{childrenField:e,disabledField:t,keyField:n}=r;return z([ae.value],{getIgnored(e){return Q(e)},getChildren(t){return t[e]},getDisabled(e){return e[t]},getKey(e){return e[n]??e.name}})}),se=t(()=>z([{}]).treeNodes[0]);function ce(){if(R.value===-1)return b(Xe,{root:!0,level:0,key:`__ellpisisGroupPlaceholder__`,internalKey:`__ellpisisGroupPlaceholder__`,title:`···`,tmNode:se.value,domId:A,isEllipsisPlaceholder:!0});let e=oe.value.treeNodes[0],t=T.value,n=!!e.children?.some(e=>t.includes(e.key));return b(Xe,{level:0,root:!0,key:`__ellpisisGroup__`,internalKey:`__ellpisisGroup__`,title:`···`,virtualChildActive:n,tmNode:e,domId:A,rawNodes:e.rawNode.children||[],tmNodes:e.children||[],isEllipsisPlaceholder:!0})}return{mergedClsPrefix:i,controlledExpandedKeys:x,uncontrolledExpanededKeys:_,mergedExpandedKeys:S,uncontrolledValue:m,mergedValue:g,activePath:T,tmNodes:w,mergedTheme:o,mergedCollapsed:l,cssVars:a?void 0:te,themeClass:k?.themeClass,overflowRef:j,counterRef:M,updateCounter:()=>{},onResize:L,onUpdateOverflow:ie,onUpdateCount:re,renderCounter:ce,getCounter:ne,onRender:k?.onRender,showOption:O,deriveResponsiveState:L}},render(){let{mergedClsPrefix:e,mode:t,themeClass:n,onRender:r}=this;r?.();let i=()=>this.tmNodes.map(e=>$(e,this.$props)),a=t===`horizontal`&&this.responsive,o=()=>b(`div`,x(this.$attrs,{role:t===`horizontal`?`menubar`:`menu`,class:[`${e}-menu`,n,`${e}-menu--${t}`,a&&`${e}-menu--responsive`,this.mergedCollapsed&&`${e}-menu--collapsed`],style:this.cssVars}),a?b(re,{ref:`overflowRef`,onUpdateOverflow:this.onUpdateOverflow,getCounter:this.getCounter,onUpdateCount:this.onUpdateCount,updateCounter:this.updateCounter,style:{width:`100%`,display:`flex`,overflow:`hidden`}},{default:i,counter:this.renderCounter}):i());return a?b(ee,{onResize:this.onResize},{default:o}):o()}}),$e={class:`header-inner`},et={class:`header-right`},tt={class:`user-email`},nt=ue(O({__name:`AppLayout`,setup(e){let n=fe(),a=pe(),c=he(),l=s(!1),u=[{label:`Dashboard`,key:`dashboard`},{label:`统计`,key:`stats`},{label:`日志`,key:`logs`},{label:`API Key`,key:`keys`},{label:`账号`,key:`account`}],d=[{label:`模型管理`,key:`models`},{label:`配额管理`,key:`quotas`},{label:`用户管理`,key:`users`},{label:`系统设置`,key:`settings`}],f=t(()=>c.isAdmin?[...u,...d]:u),p=t(()=>n.name||`dashboard`);function h(e){a.push({name:e})}async function g(){await c.logout(),a.push(`/login`)}return(e,t)=>{let n=r(`router-view`);return ne(),j(o(K),{position:`absolute`,class:`app-layout`},{default:i(()=>[v(o(Oe),{bordered:``,class:`app-header`},{default:i(()=>[S(`div`,$e,[t[3]||=S(`div`,{class:`app-name`},`carryAPI 控制台`,-1),S(`div`,et,[S(`span`,tt,m(o(c).user?.email),1),v(o(k),{size:`small`,onClick:g},{default:i(()=>[...t[2]||=[T(`退出登录`,-1)]]),_:1})])])]),_:1}),v(o(K),{"has-sider":``,position:`absolute`,class:`app-body`},{default:i(()=>[v(o(Ne),{bordered:``,"collapse-mode":`width`,"collapsed-width":64,width:200,collapsed:l.value,"show-trigger":``,onCollapse:t[0]||=e=>l.value=!0,onExpand:t[1]||=e=>l.value=!1},{default:i(()=>[v(o(Qe),{value:p.value,options:f.value,"onUpdate:value":h},null,8,[`value`,`options`])]),_:1},8,[`collapsed`]),v(o(K),{"content-style":`padding: 16px; overflow: auto;`},{default:i(()=>[v(n)]),_:1})]),_:1})]),_:1})}}}),[[`__scopeId`,`data-v-b29d5672`]]);export{nt as default};