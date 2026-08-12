import{$ as e,$t as t,Bn as n,Dn as r,Dt as i,Ft as a,In as o,It as s,K as c,Kt as l,Lt as u,Mt as d,Nt as f,Ot as p,Qt as m,Rt as h,Tn as g,U as _,Wt as v,X as y,Yt as b,_n as ee,bn as x,cn as S,ct as C,dn as w,dt as T,et as te,fn as E,g as D,gt as O,mt as k,on as A,u as j,un as M,ut as N,v as ne,wn as P,yt as re}from"./http-BFtIegGs.js";import{n as F,t as I}from"./render-B6RXEyT6.js";import{n as L,r as ie,t as R}from"./cssr-mj7kLlsn.js";import{n as ae}from"./_plugin-vue_export-helper-CEWy_VHC.js";import{t as oe}from"./use-compitable-qn4jAEEw.js";import{t as z}from"./Add-DSkGbuIc.js";import{c as B,m as V}from"./index-BPFiK6HQ.js";var se=R(`.v-x-scroll`,{overflow:`auto`,scrollbarWidth:`none`},[R(`&::-webkit-scrollbar`,{width:0,height:0})]),ce=A({name:`XScroll`,props:{disabled:Boolean,onScroll:Function},setup(){let e=o(null);function t(e){!(e.currentTarget.offsetWidth<e.currentTarget.scrollWidth)||e.deltaY===0||(e.currentTarget.scrollLeft+=e.deltaY+e.deltaX,e.preventDefault())}let n=O();return se.mount({id:`vueuc/x-scroll`,head:!0,anchorMetaName:L,ssr:n}),Object.assign({selfRef:e,handleWheel:t},{scrollTo(...t){var n;(n=e.value)==null||n.scrollTo(...t)}})},render(){return S(`div`,{ref:`selfRef`,onScroll:this.onScroll,onWheel:this.disabled?void 0:this.handleWheel,class:`v-x-scroll`},this.$slots)}}),le=/\s/;function H(e){for(var t=e.length;t--&&le.test(e.charAt(t)););return t}var U=/^\s+/;function ue(e){return e&&e.slice(0,H(e)+1).replace(U,``)}var W=NaN,de=/^[-+]0x[0-9a-f]+$/i,G=/^0b[01]+$/i,K=/^0o[0-7]+$/i,q=parseInt;function J(e){if(typeof e==`number`)return e;if(c(e))return W;if(_(e)){var t=typeof e.valueOf==`function`?e.valueOf():e;e=_(t)?t+``:t}if(typeof e!=`string`)return e===0?e:+e;e=ue(e);var n=G.test(e);return n||K.test(e)?q(e.slice(2),n?2:8):de.test(e)?W:+e}var Y=function(){return y.Date.now()},fe=`Expected a function`,pe=Math.max,X=Math.min;function me(e,t,n){var r,i,a,o,s,c,l=0,u=!1,d=!1,f=!0;if(typeof e!=`function`)throw TypeError(fe);t=J(t)||0,_(n)&&(u=!!n.leading,d=`maxWait`in n,a=d?pe(J(n.maxWait)||0,t):a,f=`trailing`in n?!!n.trailing:f);function p(t){var n=r,a=i;return r=i=void 0,l=t,o=e.apply(a,n),o}function m(e){return l=e,s=setTimeout(v,t),u?p(e):o}function h(e){var n=e-c,r=e-l,i=t-n;return d?X(i,a-r):i}function g(e){var n=e-c,r=e-l;return c===void 0||n>=t||n<0||d&&r>=a}function v(){var e=Y();if(g(e))return y(e);s=setTimeout(v,h(e))}function y(e){return s=void 0,f&&r?p(e):(r=i=void 0,o)}function b(){s!==void 0&&clearTimeout(s),l=0,r=c=i=s=void 0}function ee(){return s===void 0?o:y(Y())}function x(){var e=Y(),n=g(e);if(r=arguments,i=this,c=e,n){if(s===void 0)return m(c);if(d)return clearTimeout(s),s=setTimeout(v,t),p(c)}return s===void 0&&(s=setTimeout(v,t)),o}return x.cancel=b,x.flush=ee,x}var he=`Expected a function`;function ge(e,t,n){var r=!0,i=!0;if(typeof e!=`function`)throw TypeError(he);return _(n)&&(r=`leading`in n?!!n.leading:r,i=`trailing`in n?!!n.trailing:i),me(e,t,{leading:r,maxWait:t,trailing:i})}var Z={tabFontSizeSmall:`14px`,tabFontSizeMedium:`14px`,tabFontSizeLarge:`16px`,tabGapSmallLine:`36px`,tabGapMediumLine:`36px`,tabGapLargeLine:`36px`,tabGapSmallLineVertical:`8px`,tabGapMediumLineVertical:`8px`,tabGapLargeLineVertical:`8px`,tabPaddingSmallLine:`6px 0`,tabPaddingMediumLine:`10px 0`,tabPaddingLargeLine:`14px 0`,tabPaddingVerticalSmallLine:`6px 12px`,tabPaddingVerticalMediumLine:`8px 16px`,tabPaddingVerticalLargeLine:`10px 20px`,tabGapSmallBar:`36px`,tabGapMediumBar:`36px`,tabGapLargeBar:`36px`,tabGapSmallBarVertical:`8px`,tabGapMediumBarVertical:`8px`,tabGapLargeBarVertical:`8px`,tabPaddingSmallBar:`4px 0`,tabPaddingMediumBar:`6px 0`,tabPaddingLargeBar:`10px 0`,tabPaddingVerticalSmallBar:`6px 12px`,tabPaddingVerticalMediumBar:`8px 16px`,tabPaddingVerticalLargeBar:`10px 20px`,tabGapSmallCard:`4px`,tabGapMediumCard:`4px`,tabGapLargeCard:`4px`,tabGapSmallCardVertical:`4px`,tabGapMediumCardVertical:`4px`,tabGapLargeCardVertical:`4px`,tabPaddingSmallCard:`8px 16px`,tabPaddingMediumCard:`10px 20px`,tabPaddingLargeCard:`12px 24px`,tabPaddingSmallSegment:`4px 0`,tabPaddingMediumSegment:`6px 0`,tabPaddingLargeSegment:`8px 0`,tabPaddingVerticalLargeSegment:`0 8px`,tabPaddingVerticalSmallCard:`8px 12px`,tabPaddingVerticalMediumCard:`10px 16px`,tabPaddingVerticalLargeCard:`12px 20px`,tabPaddingVerticalSmallSegment:`0 4px`,tabPaddingVerticalMediumSegment:`0 6px`,tabGapSmallSegment:`0`,tabGapMediumSegment:`0`,tabGapLargeSegment:`0`,tabGapSmallSegmentVertical:`0`,tabGapMediumSegmentVertical:`0`,tabGapLargeSegmentVertical:`0`,panePaddingSmall:`8px 0 0 0`,panePaddingMedium:`12px 0 0 0`,panePaddingLarge:`16px 0 0 0`,closeSize:`18px`,closeIconSize:`14px`};function _e(e){let{textColor2:t,primaryColor:n,textColorDisabled:r,closeIconColor:i,closeIconColorHover:a,closeIconColorPressed:o,closeColorHover:s,closeColorPressed:c,tabColor:l,baseColor:u,dividerColor:d,fontWeight:f,textColor1:p,borderRadius:m,fontSize:h,fontWeightStrong:g}=e;return Object.assign(Object.assign({},Z),{colorSegment:l,tabFontSizeCard:h,tabTextColorLine:p,tabTextColorActiveLine:n,tabTextColorHoverLine:n,tabTextColorDisabledLine:r,tabTextColorSegment:p,tabTextColorActiveSegment:t,tabTextColorHoverSegment:t,tabTextColorDisabledSegment:r,tabTextColorBar:p,tabTextColorActiveBar:n,tabTextColorHoverBar:n,tabTextColorDisabledBar:r,tabTextColorCard:p,tabTextColorHoverCard:p,tabTextColorActiveCard:n,tabTextColorDisabledCard:r,barColor:n,closeIconColor:i,closeIconColorHover:a,closeIconColorPressed:o,closeColorHover:s,closeColorPressed:c,closeBorderRadius:m,tabColor:l,tabColorSegment:u,tabBorderColor:d,tabFontWeightActive:f,tabFontWeight:f,tabBorderRadius:m,paneTextColor:t,fontWeightStrong:g})}var ve={name:`Tabs`,common:j,self:_e},ye=re(`n-tabs`),be={tab:[String,Number,Object,Function],name:{type:[String,Number],required:!0},disabled:Boolean,displayDirective:{type:String,default:`if`},closable:{type:Boolean,default:void 0},tabProps:Object,label:[String,Number,Object,Function]},xe=A({__TAB_PANE__:!0,name:`TabPane`,alias:[`TabPanel`],props:be,slots:Object,setup(e){let t=M(ye,null);return t||T(`tab-pane`,"`n-tab-pane` must be placed inside `n-tabs`."),{style:t.paneStyleRef,class:t.paneClassRef,mergedClsPrefix:t.mergedClsPrefixRef}},render(){return S(`div`,{class:[`${this.mergedClsPrefix}-tab-pane`,this.class],style:this.style},this.$slots)}}),Q=Object.assign({internalLeftPadded:Boolean,internalAddable:Boolean,internalCreatedByPane:Boolean},V(be,[`displayDirective`])),$=A({__TAB__:!0,inheritAttrs:!1,name:`Tab`,props:Q,setup(e){let{mergedClsPrefixRef:n,valueRef:r,typeRef:i,closableRef:a,tabStyleRef:o,addTabStyleRef:s,tabClassRef:c,addTabClassRef:l,tabChangeIdRef:u,onBeforeLeaveRef:d,triggerRef:f,handleAdd:p,activateTab:m,handleClose:h}=M(ye);return{trigger:f,mergedClosable:t(()=>{if(e.internalAddable)return!1;let{closable:t}=e;return t===void 0?a.value:t}),style:o,addStyle:s,tabClass:c,addTabClass:l,clsPrefix:n,value:r,type:i,handleClose(t){t.stopPropagation(),!e.disabled&&h(e.name)},activateTab(){if(e.disabled)return;if(e.internalAddable){p();return}let{name:t}=e,n=++u.id;if(t!==r.value){let{value:i}=d;i?Promise.resolve(i(e.name,r.value)).then(e=>{e&&u.id===n&&m(t)}):m(t)}}}},render(){let{internalAddable:e,clsPrefix:t,name:n,disabled:r,label:i,tab:a,value:o,mergedClosable:s,trigger:c,$slots:{default:l}}=this,u=i??a;return S(`div`,{class:`${t}-tabs-tab-wrapper`},this.internalLeftPadded?S(`div`,{class:`${t}-tabs-tab-pad`}):null,S(`div`,Object.assign({key:n,"data-name":n,"data-disabled":r?!0:void 0},w({class:[`${t}-tabs-tab`,o===n&&`${t}-tabs-tab--active`,r&&`${t}-tabs-tab--disabled`,s&&`${t}-tabs-tab--closable`,e&&`${t}-tabs-tab--addable`,e?this.addTabClass:this.tabClass],onClick:c===`click`?this.activateTab:void 0,onMouseenter:c===`hover`?this.activateTab:void 0,style:e?this.addStyle:this.style},this.internalCreatedByPane?this.tabProps||{}:this.$attrs)),S(`span`,{class:`${t}-tabs-tab__label`},e?S(b,null,S(`div`,{class:`${t}-tabs-tab__height-placeholder`},`\xA0`),S(D,{clsPrefix:t},{default:()=>S(z,null)})):l?l():typeof u==`object`?u:I(u??n)),s&&this.type===`card`?S(B,{clsPrefix:t,class:`${t}-tabs-tab__close`,onClick:this.handleClose,disabled:r}):null))}}),Se=f(`tabs`,`
 box-sizing: border-box;
 width: 100%;
 display: flex;
 flex-direction: column;
 transition:
 background-color .3s var(--n-bezier),
 border-color .3s var(--n-bezier);
`,[s(`segment-type`,[f(`tabs-rail`,[d(`&.transition-disabled`,[f(`tabs-capsule`,`
 transition: none;
 `)])])]),s(`top`,[f(`tab-pane`,`
 padding: var(--n-pane-padding-top) var(--n-pane-padding-right) var(--n-pane-padding-bottom) var(--n-pane-padding-left);
 `)]),s(`left`,[f(`tab-pane`,`
 padding: var(--n-pane-padding-right) var(--n-pane-padding-bottom) var(--n-pane-padding-left) var(--n-pane-padding-top);
 `)]),s(`left, right`,`
 flex-direction: row;
 `,[f(`tabs-bar`,`
 width: 2px;
 right: 0;
 transition:
 top .2s var(--n-bezier),
 max-height .2s var(--n-bezier),
 background-color .3s var(--n-bezier);
 `),f(`tabs-tab`,`
 padding: var(--n-tab-padding-vertical); 
 `)]),s(`right`,`
 flex-direction: row-reverse;
 `,[f(`tab-pane`,`
 padding: var(--n-pane-padding-left) var(--n-pane-padding-top) var(--n-pane-padding-right) var(--n-pane-padding-bottom);
 `),f(`tabs-bar`,`
 left: 0;
 `)]),s(`bottom`,`
 flex-direction: column-reverse;
 justify-content: flex-end;
 `,[f(`tab-pane`,`
 padding: var(--n-pane-padding-bottom) var(--n-pane-padding-right) var(--n-pane-padding-top) var(--n-pane-padding-left);
 `),f(`tabs-bar`,`
 top: 0;
 `)]),f(`tabs-rail`,`
 position: relative;
 padding: 3px;
 border-radius: var(--n-tab-border-radius);
 width: 100%;
 background-color: var(--n-color-segment);
 transition: background-color .3s var(--n-bezier);
 display: flex;
 align-items: center;
 `,[f(`tabs-capsule`,`
 border-radius: var(--n-tab-border-radius);
 position: absolute;
 pointer-events: none;
 background-color: var(--n-tab-color-segment);
 box-shadow: 0 1px 3px 0 rgba(0, 0, 0, .08);
 transition: transform 0.3s var(--n-bezier);
 `),f(`tabs-tab-wrapper`,`
 flex-basis: 0;
 flex-grow: 1;
 display: flex;
 align-items: center;
 justify-content: center;
 `,[f(`tabs-tab`,`
 overflow: hidden;
 border-radius: var(--n-tab-border-radius);
 width: 100%;
 display: flex;
 align-items: center;
 justify-content: center;
 `,[s(`active`,`
 font-weight: var(--n-font-weight-strong);
 color: var(--n-tab-text-color-active);
 `),d(`&:hover`,`
 color: var(--n-tab-text-color-hover);
 `)])])]),s(`flex`,[f(`tabs-nav`,`
 width: 100%;
 position: relative;
 `,[f(`tabs-wrapper`,`
 width: 100%;
 `,[f(`tabs-tab`,`
 margin-right: 0;
 `)])])]),f(`tabs-nav`,`
 box-sizing: border-box;
 line-height: 1.5;
 display: flex;
 transition: border-color .3s var(--n-bezier);
 `,[a(`prefix, suffix`,`
 display: flex;
 align-items: center;
 `),a(`prefix`,`padding-right: 16px;`),a(`suffix`,`padding-left: 16px;`)]),s(`top, bottom`,[d(`>`,[f(`tabs-nav`,[f(`tabs-nav-scroll-wrapper`,[d(`&::before`,`
 top: 0;
 bottom: 0;
 left: 0;
 width: 20px;
 `),d(`&::after`,`
 top: 0;
 bottom: 0;
 right: 0;
 width: 20px;
 `),s(`shadow-start`,[d(`&::before`,`
 box-shadow: inset 10px 0 8px -8px rgba(0, 0, 0, .12);
 `)]),s(`shadow-end`,[d(`&::after`,`
 box-shadow: inset -10px 0 8px -8px rgba(0, 0, 0, .12);
 `)])])])])]),s(`left, right`,[f(`tabs-nav-scroll-content`,`
 flex-direction: column;
 `),d(`>`,[f(`tabs-nav`,[f(`tabs-nav-scroll-wrapper`,[d(`&::before`,`
 top: 0;
 left: 0;
 right: 0;
 height: 20px;
 `),d(`&::after`,`
 bottom: 0;
 left: 0;
 right: 0;
 height: 20px;
 `),s(`shadow-start`,[d(`&::before`,`
 box-shadow: inset 0 10px 8px -8px rgba(0, 0, 0, .12);
 `)]),s(`shadow-end`,[d(`&::after`,`
 box-shadow: inset 0 -10px 8px -8px rgba(0, 0, 0, .12);
 `)])])])])]),f(`tabs-nav-scroll-wrapper`,`
 flex: 1;
 position: relative;
 overflow: hidden;
 `,[f(`tabs-nav-y-scroll`,`
 height: 100%;
 width: 100%;
 overflow-y: auto; 
 scrollbar-width: none;
 `,[d(`&::-webkit-scrollbar, &::-webkit-scrollbar-track-piece, &::-webkit-scrollbar-thumb`,`
 width: 0;
 height: 0;
 display: none;
 `)]),d(`&::before, &::after`,`
 transition: box-shadow .3s var(--n-bezier);
 pointer-events: none;
 content: "";
 position: absolute;
 z-index: 1;
 `)]),f(`tabs-nav-scroll-content`,`
 display: flex;
 position: relative;
 min-width: 100%;
 min-height: 100%;
 width: fit-content;
 box-sizing: border-box;
 `),f(`tabs-wrapper`,`
 display: inline-flex;
 flex-wrap: nowrap;
 position: relative;
 `),f(`tabs-tab-wrapper`,`
 display: flex;
 flex-wrap: nowrap;
 flex-shrink: 0;
 flex-grow: 0;
 `),f(`tabs-tab`,`
 cursor: pointer;
 white-space: nowrap;
 flex-wrap: nowrap;
 display: inline-flex;
 align-items: center;
 color: var(--n-tab-text-color);
 font-size: var(--n-tab-font-size);
 background-clip: padding-box;
 padding: var(--n-tab-padding);
 transition:
 box-shadow .3s var(--n-bezier),
 color .3s var(--n-bezier),
 background-color .3s var(--n-bezier),
 border-color .3s var(--n-bezier);
 `,[s(`disabled`,{cursor:`not-allowed`}),a(`close`,`
 margin-left: 6px;
 transition:
 background-color .3s var(--n-bezier),
 color .3s var(--n-bezier);
 `),a(`label`,`
 display: flex;
 align-items: center;
 z-index: 1;
 `)]),f(`tabs-bar`,`
 position: absolute;
 bottom: 0;
 height: 2px;
 border-radius: 1px;
 background-color: var(--n-bar-color);
 transition:
 left .2s var(--n-bezier),
 max-width .2s var(--n-bezier),
 opacity .3s var(--n-bezier),
 background-color .3s var(--n-bezier);
 `,[d(`&.transition-disabled`,`
 transition: none;
 `),s(`disabled`,`
 background-color: var(--n-tab-text-color-disabled)
 `)]),f(`tabs-pane-wrapper`,`
 position: relative;
 overflow: hidden;
 transition: max-height .2s var(--n-bezier);
 `),f(`tab-pane`,`
 color: var(--n-pane-text-color);
 width: 100%;
 transition:
 color .3s var(--n-bezier),
 background-color .3s var(--n-bezier),
 opacity .2s var(--n-bezier);
 left: 0;
 right: 0;
 top: 0;
 `,[d(`&.next-transition-leave-active, &.prev-transition-leave-active, &.next-transition-enter-active, &.prev-transition-enter-active`,`
 transition:
 color .3s var(--n-bezier),
 background-color .3s var(--n-bezier),
 transform .2s var(--n-bezier),
 opacity .2s var(--n-bezier);
 `),d(`&.next-transition-leave-active, &.prev-transition-leave-active`,`
 position: absolute;
 `),d(`&.next-transition-enter-from, &.prev-transition-leave-to`,`
 transform: translateX(32px);
 opacity: 0;
 `),d(`&.next-transition-leave-to, &.prev-transition-enter-from`,`
 transform: translateX(-32px);
 opacity: 0;
 `),d(`&.next-transition-leave-from, &.next-transition-enter-to, &.prev-transition-leave-from, &.prev-transition-enter-to`,`
 transform: translateX(0);
 opacity: 1;
 `)]),f(`tabs-tab-pad`,`
 box-sizing: border-box;
 width: var(--n-tab-gap);
 flex-grow: 0;
 flex-shrink: 0;
 `),s(`line-type, bar-type`,[f(`tabs-tab`,`
 font-weight: var(--n-tab-font-weight);
 box-sizing: border-box;
 vertical-align: bottom;
 `,[d(`&:hover`,{color:`var(--n-tab-text-color-hover)`}),s(`active`,`
 color: var(--n-tab-text-color-active);
 font-weight: var(--n-tab-font-weight-active);
 `),s(`disabled`,{color:`var(--n-tab-text-color-disabled)`})])]),f(`tabs-nav`,[s(`line-type`,[s(`top`,[a(`prefix, suffix`,`
 border-bottom: 1px solid var(--n-tab-border-color);
 `),f(`tabs-nav-scroll-content`,`
 border-bottom: 1px solid var(--n-tab-border-color);
 `),f(`tabs-bar`,`
 bottom: -1px;
 `)]),s(`left`,[a(`prefix, suffix`,`
 border-right: 1px solid var(--n-tab-border-color);
 `),f(`tabs-nav-scroll-content`,`
 border-right: 1px solid var(--n-tab-border-color);
 `),f(`tabs-bar`,`
 right: -1px;
 `)]),s(`right`,[a(`prefix, suffix`,`
 border-left: 1px solid var(--n-tab-border-color);
 `),f(`tabs-nav-scroll-content`,`
 border-left: 1px solid var(--n-tab-border-color);
 `),f(`tabs-bar`,`
 left: -1px;
 `)]),s(`bottom`,[a(`prefix, suffix`,`
 border-top: 1px solid var(--n-tab-border-color);
 `),f(`tabs-nav-scroll-content`,`
 border-top: 1px solid var(--n-tab-border-color);
 `),f(`tabs-bar`,`
 top: -1px;
 `)]),a(`prefix, suffix`,`
 transition: border-color .3s var(--n-bezier);
 `),f(`tabs-nav-scroll-content`,`
 transition: border-color .3s var(--n-bezier);
 `),f(`tabs-bar`,`
 border-radius: 0;
 `)]),s(`card-type`,[a(`prefix, suffix`,`
 transition: border-color .3s var(--n-bezier);
 `),f(`tabs-pad`,`
 flex-grow: 1;
 transition: border-color .3s var(--n-bezier);
 `),f(`tabs-tab-pad`,`
 transition: border-color .3s var(--n-bezier);
 `),f(`tabs-tab`,`
 font-weight: var(--n-tab-font-weight);
 border: 1px solid var(--n-tab-border-color);
 background-color: var(--n-tab-color);
 box-sizing: border-box;
 position: relative;
 vertical-align: bottom;
 display: flex;
 justify-content: space-between;
 font-size: var(--n-tab-font-size);
 color: var(--n-tab-text-color);
 `,[s(`addable`,`
 padding-left: 8px;
 padding-right: 8px;
 font-size: 16px;
 justify-content: center;
 `,[a(`height-placeholder`,`
 width: 0;
 font-size: var(--n-tab-font-size);
 `),u(`disabled`,[d(`&:hover`,`
 color: var(--n-tab-text-color-hover);
 `)])]),s(`closable`,`padding-right: 8px;`),s(`active`,`
 background-color: #0000;
 font-weight: var(--n-tab-font-weight-active);
 color: var(--n-tab-text-color-active);
 `),s(`disabled`,`color: var(--n-tab-text-color-disabled);`)])]),s(`left, right`,`
 flex-direction: column; 
 `,[a(`prefix, suffix`,`
 padding: var(--n-tab-padding-vertical);
 `),f(`tabs-wrapper`,`
 flex-direction: column;
 `),f(`tabs-tab-wrapper`,`
 flex-direction: column;
 `,[f(`tabs-tab-pad`,`
 height: var(--n-tab-gap-vertical);
 width: 100%;
 `)])]),s(`top`,[s(`card-type`,[f(`tabs-scroll-padding`,`border-bottom: 1px solid var(--n-tab-border-color);`),a(`prefix, suffix`,`
 border-bottom: 1px solid var(--n-tab-border-color);
 `),f(`tabs-tab`,`
 border-top-left-radius: var(--n-tab-border-radius);
 border-top-right-radius: var(--n-tab-border-radius);
 `,[s(`active`,`
 border-bottom: 1px solid #0000;
 `)]),f(`tabs-tab-pad`,`
 border-bottom: 1px solid var(--n-tab-border-color);
 `),f(`tabs-pad`,`
 border-bottom: 1px solid var(--n-tab-border-color);
 `)])]),s(`left`,[s(`card-type`,[f(`tabs-scroll-padding`,`border-right: 1px solid var(--n-tab-border-color);`),a(`prefix, suffix`,`
 border-right: 1px solid var(--n-tab-border-color);
 `),f(`tabs-tab`,`
 border-top-left-radius: var(--n-tab-border-radius);
 border-bottom-left-radius: var(--n-tab-border-radius);
 `,[s(`active`,`
 border-right: 1px solid #0000;
 `)]),f(`tabs-tab-pad`,`
 border-right: 1px solid var(--n-tab-border-color);
 `),f(`tabs-pad`,`
 border-right: 1px solid var(--n-tab-border-color);
 `)])]),s(`right`,[s(`card-type`,[f(`tabs-scroll-padding`,`border-left: 1px solid var(--n-tab-border-color);`),a(`prefix, suffix`,`
 border-left: 1px solid var(--n-tab-border-color);
 `),f(`tabs-tab`,`
 border-top-right-radius: var(--n-tab-border-radius);
 border-bottom-right-radius: var(--n-tab-border-radius);
 `,[s(`active`,`
 border-left: 1px solid #0000;
 `)]),f(`tabs-tab-pad`,`
 border-left: 1px solid var(--n-tab-border-color);
 `),f(`tabs-pad`,`
 border-left: 1px solid var(--n-tab-border-color);
 `)])]),s(`bottom`,[s(`card-type`,[f(`tabs-scroll-padding`,`border-top: 1px solid var(--n-tab-border-color);`),a(`prefix, suffix`,`
 border-top: 1px solid var(--n-tab-border-color);
 `),f(`tabs-tab`,`
 border-bottom-left-radius: var(--n-tab-border-radius);
 border-bottom-right-radius: var(--n-tab-border-radius);
 `,[s(`active`,`
 border-top: 1px solid #0000;
 `)]),f(`tabs-tab-pad`,`
 border-top: 1px solid var(--n-tab-border-color);
 `),f(`tabs-pad`,`
 border-top: 1px solid var(--n-tab-border-color);
 `)])])])]),Ce=ge,we=Object.assign(Object.assign({},ne.props),{value:[String,Number],defaultValue:[String,Number],trigger:{type:String,default:`click`},type:{type:String,default:`bar`},closable:Boolean,justifyContent:String,size:String,placement:{type:String,default:`top`},tabStyle:[String,Object],tabClass:String,addTabStyle:[String,Object],addTabClass:String,barWidth:Number,paneClass:String,paneStyle:[String,Object],paneWrapperClass:String,paneWrapperStyle:[String,Object],addable:[Boolean,Object],tabsPadding:{type:Number,default:0},animated:Boolean,onBeforeLeave:Function,onAdd:Function,"onUpdate:value":[Function,Array],onUpdateValue:[Function,Array],onClose:[Function,Array],labelSize:String,activeName:[String,Number],onActiveNameChange:[Function,Array]}),Te=A({name:`Tabs`,props:we,slots:Object,setup(r,{slots:a}){let{mergedClsPrefixRef:s,inlineThemeDisabled:c,mergedComponentPropsRef:l}=te(r),u=ne(`Tabs`,`-tabs`,Se,ve,r,s),d=o(null),f=o(null),m=o(null),_=o(null),v=o(null),y=o(null),b=o(!0),S=o(!0),C=oe(r,[`labelSize`,`size`]),w=t(()=>C.value?C.value:l?.value?.Tabs?.size||`medium`),T=oe(r,[`activeName`,`value`]),D=o(T.value??r.defaultValue??(a.default?F(a.default())[0]?.props?.name:null)),O=ae(T,D),k={id:0},A=t(()=>{if(!(!r.justifyContent||r.type===`card`))return{display:`flex`,justifyContent:r.justifyContent}});P(O,()=>{k.id=0,L(),R()});function j(){let{value:e}=O;return e===null?null:d.value?.querySelector(`[data-name="${e}"]`)}function M(e){if(r.type===`card`)return;let{value:t}=f;if(!t)return;let n=t.style.opacity===`0`;if(e){let i=`${s.value}-tabs-bar--disabled`,{barWidth:a,placement:o}=r;if(e.dataset.disabled===`true`?t.classList.add(i):t.classList.remove(i),[`top`,`bottom`].includes(o)){if(I([`top`,`maxHeight`,`height`]),typeof a==`number`&&e.offsetWidth>=a){let n=Math.floor((e.offsetWidth-a)/2)+e.offsetLeft;t.style.left=`${n}px`,t.style.maxWidth=`${a}px`}else t.style.left=`${e.offsetLeft}px`,t.style.maxWidth=`${e.offsetWidth}px`;t.style.width=`8192px`,n&&(t.style.transition=`none`),t.offsetWidth,n&&(t.style.transition=``,t.style.opacity=`1`)}else{if(I([`left`,`maxWidth`,`width`]),typeof a==`number`&&e.offsetHeight>=a){let n=Math.floor((e.offsetHeight-a)/2)+e.offsetTop;t.style.top=`${n}px`,t.style.maxHeight=`${a}px`}else t.style.top=`${e.offsetTop}px`,t.style.maxHeight=`${e.offsetHeight}px`;t.style.height=`8192px`,n&&(t.style.transition=`none`),t.offsetHeight,n&&(t.style.transition=``,t.style.opacity=`1`)}}}function re(){if(r.type===`card`)return;let{value:e}=f;e&&(e.style.opacity=`0`)}function I(e){let{value:t}=f;if(t)for(let n of e)t.style[n]=``}function L(){if(r.type===`card`)return;let e=j();e?M(e):re()}function R(){let e=v.value?.$el;if(!e)return;let t=j();if(!t)return;let{scrollLeft:n,offsetWidth:r}=e,{offsetLeft:i,offsetWidth:a}=t;n>i?e.scrollTo({top:0,left:i,behavior:`smooth`}):i+a>n+r&&e.scrollTo({top:0,left:i+a-r,behavior:`smooth`})}let z=o(null),B=0,V=null;function se(e){let t=z.value;if(t){B=e.getBoundingClientRect().height;let n=`${B}px`,r=()=>{t.style.height=n,t.style.maxHeight=n};V?(r(),V(),V=null):V=r}}function ce(e){let t=z.value;if(t){let n=e.getBoundingClientRect().height,r=()=>{document.body.offsetHeight,t.style.maxHeight=`${n}px`,t.style.height=`${Math.max(B,n)}px`};V?(V(),V=null,r()):V=r}}function le(){let e=z.value;if(e){e.style.maxHeight=``,e.style.height=``;let{paneWrapperStyle:t}=r;if(typeof t==`string`)e.style.cssText=t;else if(t){let{maxHeight:n,height:r}=t;n!==void 0&&(e.style.maxHeight=n),r!==void 0&&(e.style.height=r)}}}let H={value:[]},U=o(`next`);function ue(e){let t=O.value,n=`next`;for(let r of H.value){if(r===t)break;if(r===e){n=`prev`;break}}U.value=n,W(e)}function W(e){let{onActiveNameChange:t,onUpdateValue:n,"onUpdate:value":i}=r;t&&N(t,e),n&&N(n,e),i&&N(i,e),D.value=e}function de(e){let{onClose:t}=r;t&&N(t,e)}let G=!0;function K(){let{value:e}=f;if(!e)return;G||=!1;let t=`transition-disabled`;e.classList.add(t),L(),e.classList.remove(t)}let q=o(null);function J({transitionDisabled:e}){let t=d.value;if(!t)return;e&&t.classList.add(`transition-disabled`);let n=j();n&&q.value&&(q.value.style.width=`${n.offsetWidth}px`,q.value.style.height=`${n.offsetHeight}px`,q.value.style.transform=`translateX(${n.offsetLeft-i(getComputedStyle(t).paddingLeft)}px)`,e&&q.value.offsetWidth),e&&t.classList.remove(`transition-disabled`)}P([O],()=>{r.type===`segment`&&E(()=>{J({transitionDisabled:!1})})}),ee(()=>{r.type===`segment`&&J({transitionDisabled:!0})});let Y=0;function fe(e){if(e.contentRect.width===0&&e.contentRect.height===0||Y===e.contentRect.width)return;Y=e.contentRect.width;let{type:t}=r;if((t===`line`||t===`bar`)&&(G||r.justifyContent?.startsWith(`space`))&&K(),t!==`segment`){let{placement:e}=r;Z((e===`top`||e===`bottom`?v.value?.$el:y.value)||null)}}let pe=Ce(fe,64);P([()=>r.justifyContent,()=>r.size],()=>{E(()=>{let{type:e}=r;(e===`line`||e===`bar`)&&K()})});let X=o(!1);function me(e){let{target:t,contentRect:{width:n,height:i}}=e,a=t.parentElement.parentElement.offsetWidth,o=t.parentElement.parentElement.offsetHeight,{placement:s}=r;if(!X.value)s===`top`||s===`bottom`?a<n&&(X.value=!0):o<i&&(X.value=!0);else{let{value:e}=_;if(!e)return;s===`top`||s===`bottom`?a-n>e.$el.offsetWidth&&(X.value=!1):o-i>e.$el.offsetHeight&&(X.value=!1)}Z(v.value?.$el||null)}let he=Ce(me,64);function ge(){let{onAdd:e}=r;e&&e(),E(()=>{let e=j(),{value:t}=v;!e||!t||t.scrollTo({left:e.offsetLeft,top:0,behavior:`smooth`})})}function Z(e){if(!e)return;let{placement:t}=r;if(t===`top`||t===`bottom`){let{scrollLeft:t,scrollWidth:n,offsetWidth:r}=e;b.value=t<=0,S.value=t+r>=n}else{let{scrollTop:t,scrollHeight:n,offsetHeight:r}=e;b.value=t<=0,S.value=t+r>=n}}let _e=Ce(e=>{Z(e.target)},64);x(ye,{triggerRef:n(r,`trigger`),tabStyleRef:n(r,`tabStyle`),tabClassRef:n(r,`tabClass`),addTabStyleRef:n(r,`addTabStyle`),addTabClassRef:n(r,`addTabClass`),paneClassRef:n(r,`paneClass`),paneStyleRef:n(r,`paneStyle`),mergedClsPrefixRef:s,typeRef:n(r,`type`),closableRef:n(r,`closable`),valueRef:O,tabChangeIdRef:k,onBeforeLeaveRef:n(r,`onBeforeLeave`),activateTab:ue,handleClose:de,handleAdd:ge}),ie(()=>{L(),R()}),g(()=>{let{value:e}=m;if(!e)return;let{value:t}=s,n=`${t}-tabs-nav-scroll-wrapper--shadow-start`,r=`${t}-tabs-nav-scroll-wrapper--shadow-end`;b.value?e.classList.remove(n):e.classList.add(n),S.value?e.classList.remove(r):e.classList.add(r)});let be={syncBarPosition:()=>{L()}},xe=()=>{J({transitionDisabled:!0})},Q=t(()=>{let{value:e}=w,{type:t}=r,n=`${e}${{card:`Card`,bar:`Bar`,line:`Line`,segment:`Segment`}[t]}`,{self:{barColor:i,closeIconColor:a,closeIconColorHover:o,closeIconColorPressed:s,tabColor:c,tabBorderColor:l,paneTextColor:d,tabFontWeight:f,tabBorderRadius:m,tabFontWeightActive:g,colorSegment:_,fontWeightStrong:v,tabColorSegment:y,closeSize:b,closeIconSize:ee,closeColorHover:x,closeColorPressed:S,closeBorderRadius:C,[h(`panePadding`,e)]:T,[h(`tabPadding`,n)]:te,[h(`tabPaddingVertical`,n)]:E,[h(`tabGap`,n)]:D,[h(`tabGap`,`${n}Vertical`)]:O,[h(`tabTextColor`,t)]:k,[h(`tabTextColorActive`,t)]:A,[h(`tabTextColorHover`,t)]:j,[h(`tabTextColorDisabled`,t)]:M,[h(`tabFontSize`,e)]:N},common:{cubicBezierEaseInOut:ne}}=u.value;return{"--n-bezier":ne,"--n-color-segment":_,"--n-bar-color":i,"--n-tab-font-size":N,"--n-tab-text-color":k,"--n-tab-text-color-active":A,"--n-tab-text-color-disabled":M,"--n-tab-text-color-hover":j,"--n-pane-text-color":d,"--n-tab-border-color":l,"--n-tab-border-radius":m,"--n-close-size":b,"--n-close-icon-size":ee,"--n-close-color-hover":x,"--n-close-color-pressed":S,"--n-close-border-radius":C,"--n-close-icon-color":a,"--n-close-icon-color-hover":o,"--n-close-icon-color-pressed":s,"--n-tab-color":c,"--n-tab-font-weight":f,"--n-tab-font-weight-active":g,"--n-tab-padding":te,"--n-tab-padding-vertical":E,"--n-tab-gap":D,"--n-tab-gap-vertical":O,"--n-pane-padding-left":p(T,`left`),"--n-pane-padding-right":p(T,`right`),"--n-pane-padding-top":p(T,`top`),"--n-pane-padding-bottom":p(T,`bottom`),"--n-font-weight-strong":v,"--n-tab-color-segment":y}}),$=c?e(`tabs`,t(()=>`${w.value[0]}${r.type[0]}`),Q,r):void 0;return Object.assign({mergedClsPrefix:s,mergedValue:O,renderedNames:new Set,segmentCapsuleElRef:q,tabsPaneWrapperRef:z,tabsElRef:d,barElRef:f,addTabInstRef:_,xScrollInstRef:v,scrollWrapperElRef:m,addTabFixed:X,tabWrapperStyle:A,handleNavResize:pe,mergedSize:w,handleScroll:_e,handleTabsResize:he,cssVars:c?void 0:Q,themeClass:$?.themeClass,animationDirection:U,renderNameListRef:H,yScrollElRef:y,handleSegmentResize:xe,onAnimationBeforeLeave:se,onAnimationEnter:ce,onAnimationAfterEnter:le,onRender:$?.onRender},be)},render(){let{mergedClsPrefix:e,type:t,placement:n,addTabFixed:r,addable:i,mergedSize:a,renderNameListRef:o,onRender:s,paneWrapperClass:c,paneWrapperStyle:l,$slots:{default:u,prefix:d,suffix:f}}=this;s?.();let p=u?F(u()).filter(e=>e.type.__TAB_PANE__===!0):[],m=u?F(u()).filter(e=>e.type.__TAB__===!0):[],h=!m.length,g=t===`card`,_=t===`segment`,v=!g&&!_&&this.justifyContent;o.value=[];let y=()=>{let t=S(`div`,{style:this.tabWrapperStyle,class:`${e}-tabs-wrapper`},v?null:S(`div`,{class:`${e}-tabs-scroll-padding`,style:n===`top`||n===`bottom`?{width:`${this.tabsPadding}px`}:{height:`${this.tabsPadding}px`}}),h?p.map((e,t)=>(o.value.push(e.props.name),ke(S($,Object.assign({},e.props,{internalCreatedByPane:!0,internalLeftPadded:t!==0&&(!v||v===`center`||v===`start`||v===`end`)}),e.children?{default:e.children.tab}:void 0)))):m.map((e,t)=>(o.value.push(e.props.name),ke(t!==0&&!v?Oe(e):e))),!r&&i&&g?De(i,(h?p.length:m.length)!==0):null,v?null:S(`div`,{class:`${e}-tabs-scroll-padding`,style:{width:`${this.tabsPadding}px`}}));return S(`div`,{ref:`tabsElRef`,class:`${e}-tabs-nav-scroll-content`},g&&i?S(k,{onResize:this.handleTabsResize},{default:()=>t}):t,g?S(`div`,{class:`${e}-tabs-pad`}):null,g?null:S(`div`,{ref:`barElRef`,class:`${e}-tabs-bar`}))},b=_?`top`:n;return S(`div`,{class:[`${e}-tabs`,this.themeClass,`${e}-tabs--${t}-type`,`${e}-tabs--${a}-size`,v&&`${e}-tabs--flex`,`${e}-tabs--${b}`],style:this.cssVars},S(`div`,{class:[`${e}-tabs-nav--${t}-type`,`${e}-tabs-nav--${b}`,`${e}-tabs-nav`]},C(d,t=>t&&S(`div`,{class:`${e}-tabs-nav__prefix`},t)),_?S(k,{onResize:this.handleSegmentResize},{default:()=>S(`div`,{class:`${e}-tabs-rail`,ref:`tabsElRef`},S(`div`,{class:`${e}-tabs-capsule`,ref:`segmentCapsuleElRef`},S(`div`,{class:`${e}-tabs-wrapper`},S(`div`,{class:`${e}-tabs-tab`}))),h?p.map((e,t)=>(o.value.push(e.props.name),S($,Object.assign({},e.props,{internalCreatedByPane:!0,internalLeftPadded:t!==0}),e.children?{default:e.children.tab}:void 0))):m.map((e,t)=>(o.value.push(e.props.name),t===0?e:Oe(e))))}):S(k,{onResize:this.handleNavResize},{default:()=>S(`div`,{class:`${e}-tabs-nav-scroll-wrapper`,ref:`scrollWrapperElRef`},[`top`,`bottom`].includes(b)?S(ce,{ref:`xScrollInstRef`,onScroll:this.handleScroll},{default:y}):S(`div`,{class:`${e}-tabs-nav-y-scroll`,onScroll:this.handleScroll,ref:`yScrollElRef`},y()))}),r&&i&&g?De(i,!0):null,C(f,t=>t&&S(`div`,{class:`${e}-tabs-nav__suffix`},t))),h&&(this.animated&&(b===`top`||b===`bottom`)?S(`div`,{ref:`tabsPaneWrapperRef`,style:l,class:[`${e}-tabs-pane-wrapper`,c]},Ee(p,this.mergedValue,this.renderedNames,this.onAnimationBeforeLeave,this.onAnimationEnter,this.onAnimationAfterEnter,this.animationDirection)):Ee(p,this.mergedValue,this.renderedNames)))}});function Ee(e,t,n,i,a,o,s){let c=[];return e.forEach(e=>{let{name:i,displayDirective:a,"display-directive":o}=e.props,s=e=>a===e||o===e,u=t===i;if(e.key!==void 0&&(e.key=i),u||s(`show`)||s(`show:lazy`)&&n.has(i)){n.has(i)||n.add(i);let t=!s(`if`);c.push(t?r(e,[[l,u]]):e)}}),s?S(v,{name:`${s}-transition`,onBeforeLeave:i,onEnter:a,onAfterEnter:o},{default:()=>c}):c}function De(e,t){return S($,{ref:`addTabInstRef`,key:`__addable`,name:`__addable`,internalCreatedByPane:!0,internalAddable:!0,internalLeftPadded:t,disabled:typeof e==`object`&&e.disabled})}function Oe(e){let t=m(e);return t.props?t.props.internalLeftPadded=!0:t.props={internalLeftPadded:!0},t}function ke(e){return Array.isArray(e.dynamicProps)?e.dynamicProps.includes(`internalLeftPadded`)||e.dynamicProps.push(`internalLeftPadded`):e.dynamicProps=[`internalLeftPadded`],e}export{xe as n,Te as t};