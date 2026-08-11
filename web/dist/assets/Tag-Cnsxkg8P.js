import{$ as e,$t as t,Bn as n,Ft as r,In as i,It as a,Lt as o,Mt as s,Nt as c,Ot as l,Rt as u,bn as d,cn as f,ct as p,et as m,on as h,pt as g,u as _,ut as v,v as y,wt as b,x,yt as S}from"./http-QSbWvBrg.js";import{c as C}from"./index-DvLvTovQ.js";var w={closeIconSizeTiny:`12px`,closeIconSizeSmall:`12px`,closeIconSizeMedium:`14px`,closeIconSizeLarge:`14px`,closeSizeTiny:`16px`,closeSizeSmall:`16px`,closeSizeMedium:`18px`,closeSizeLarge:`18px`,padding:`0 7px`,closeMargin:`0 0 0 4px`};function T(e){let{textColor2:t,primaryColorHover:n,primaryColorPressed:r,primaryColor:i,infoColor:a,successColor:o,warningColor:s,errorColor:c,baseColor:l,borderColor:u,opacityDisabled:d,tagColor:f,closeIconColor:p,closeIconColorHover:m,closeIconColorPressed:h,borderRadiusSmall:g,fontSizeMini:_,fontSizeTiny:v,fontSizeSmall:y,fontSizeMedium:x,heightMini:S,heightTiny:C,heightSmall:T,heightMedium:E,closeColorHover:D,closeColorPressed:O,buttonColor2Hover:k,buttonColor2Pressed:A,fontWeightStrong:j}=e;return Object.assign(Object.assign({},w),{closeBorderRadius:g,heightTiny:S,heightSmall:C,heightMedium:T,heightLarge:E,borderRadius:g,opacityDisabled:d,fontSizeTiny:_,fontSizeSmall:v,fontSizeMedium:y,fontSizeLarge:x,fontWeightStrong:j,textColorCheckable:t,textColorHoverCheckable:t,textColorPressedCheckable:t,textColorChecked:l,colorCheckable:`#0000`,colorHoverCheckable:k,colorPressedCheckable:A,colorChecked:i,colorCheckedHover:n,colorCheckedPressed:r,border:`1px solid ${u}`,textColor:t,color:f,colorBordered:`rgb(250, 250, 252)`,closeIconColor:p,closeIconColorHover:m,closeIconColorPressed:h,closeColorHover:D,closeColorPressed:O,borderPrimary:`1px solid ${b(i,{alpha:.3})}`,textColorPrimary:i,colorPrimary:b(i,{alpha:.12}),colorBorderedPrimary:b(i,{alpha:.1}),closeIconColorPrimary:i,closeIconColorHoverPrimary:i,closeIconColorPressedPrimary:i,closeColorHoverPrimary:b(i,{alpha:.12}),closeColorPressedPrimary:b(i,{alpha:.18}),borderInfo:`1px solid ${b(a,{alpha:.3})}`,textColorInfo:a,colorInfo:b(a,{alpha:.12}),colorBorderedInfo:b(a,{alpha:.1}),closeIconColorInfo:a,closeIconColorHoverInfo:a,closeIconColorPressedInfo:a,closeColorHoverInfo:b(a,{alpha:.12}),closeColorPressedInfo:b(a,{alpha:.18}),borderSuccess:`1px solid ${b(o,{alpha:.3})}`,textColorSuccess:o,colorSuccess:b(o,{alpha:.12}),colorBorderedSuccess:b(o,{alpha:.1}),closeIconColorSuccess:o,closeIconColorHoverSuccess:o,closeIconColorPressedSuccess:o,closeColorHoverSuccess:b(o,{alpha:.12}),closeColorPressedSuccess:b(o,{alpha:.18}),borderWarning:`1px solid ${b(s,{alpha:.35})}`,textColorWarning:s,colorWarning:b(s,{alpha:.15}),colorBorderedWarning:b(s,{alpha:.12}),closeIconColorWarning:s,closeIconColorHoverWarning:s,closeIconColorPressedWarning:s,closeColorHoverWarning:b(s,{alpha:.12}),closeColorPressedWarning:b(s,{alpha:.18}),borderError:`1px solid ${b(c,{alpha:.23})}`,textColorError:c,colorError:b(c,{alpha:.1}),colorBorderedError:b(c,{alpha:.08}),closeIconColorError:c,closeIconColorHoverError:c,closeIconColorPressedError:c,closeColorHoverError:b(c,{alpha:.12}),closeColorPressedError:b(c,{alpha:.18})})}var E={name:`Tag`,common:_,self:T},D={color:Object,type:{type:String,default:`default`},round:Boolean,size:String,closable:Boolean,disabled:{type:Boolean,default:void 0}},O=c(`tag`,`
 --n-close-margin: var(--n-close-margin-top) var(--n-close-margin-right) var(--n-close-margin-bottom) var(--n-close-margin-left);
 white-space: nowrap;
 position: relative;
 box-sizing: border-box;
 cursor: default;
 display: inline-flex;
 align-items: center;
 flex-wrap: nowrap;
 padding: var(--n-padding);
 border-radius: var(--n-border-radius);
 color: var(--n-text-color);
 background-color: var(--n-color);
 transition: 
 border-color .3s var(--n-bezier),
 background-color .3s var(--n-bezier),
 color .3s var(--n-bezier),
 box-shadow .3s var(--n-bezier),
 opacity .3s var(--n-bezier);
 line-height: 1;
 height: var(--n-height);
 font-size: var(--n-font-size);
`,[a(`strong`,`
 font-weight: var(--n-font-weight-strong);
 `),r(`border`,`
 pointer-events: none;
 position: absolute;
 left: 0;
 right: 0;
 top: 0;
 bottom: 0;
 border-radius: inherit;
 border: var(--n-border);
 transition: border-color .3s var(--n-bezier);
 `),r(`icon`,`
 display: flex;
 margin: 0 4px 0 0;
 color: var(--n-text-color);
 transition: color .3s var(--n-bezier);
 font-size: var(--n-avatar-size-override);
 `),r(`avatar`,`
 display: flex;
 margin: 0 6px 0 0;
 `),r(`close`,`
 margin: var(--n-close-margin);
 transition:
 background-color .3s var(--n-bezier),
 color .3s var(--n-bezier);
 `),a(`round`,`
 padding: 0 calc(var(--n-height) / 3);
 border-radius: calc(var(--n-height) / 2);
 `,[r(`icon`,`
 margin: 0 4px 0 calc((var(--n-height) - 8px) / -2);
 `),r(`avatar`,`
 margin: 0 6px 0 calc((var(--n-height) - 8px) / -2);
 `),a(`closable`,`
 padding: 0 calc(var(--n-height) / 4) 0 calc(var(--n-height) / 3);
 `)]),a(`icon, avatar`,[a(`round`,`
 padding: 0 calc(var(--n-height) / 3) 0 calc(var(--n-height) / 2);
 `)]),a(`disabled`,`
 cursor: not-allowed !important;
 opacity: var(--n-opacity-disabled);
 `),a(`checkable`,`
 cursor: pointer;
 box-shadow: none;
 color: var(--n-text-color-checkable);
 background-color: var(--n-color-checkable);
 `,[o(`disabled`,[s(`&:hover`,`background-color: var(--n-color-hover-checkable);`,[o(`checked`,`color: var(--n-text-color-hover-checkable);`)]),s(`&:active`,`background-color: var(--n-color-pressed-checkable);`,[o(`checked`,`color: var(--n-text-color-pressed-checkable);`)])]),a(`checked`,`
 color: var(--n-text-color-checked);
 background-color: var(--n-color-checked);
 `,[o(`disabled`,[s(`&:hover`,`background-color: var(--n-color-checked-hover);`),s(`&:active`,`background-color: var(--n-color-checked-pressed);`)])])])]),k=Object.assign(Object.assign(Object.assign({},y.props),D),{bordered:{type:Boolean,default:void 0},checked:Boolean,checkable:Boolean,strong:Boolean,triggerClickOnClose:Boolean,onClose:[Array,Function],onMouseenter:Function,onMouseleave:Function,"onUpdate:checked":Function,onUpdateChecked:Function,internalCloseFocusable:{type:Boolean,default:!0},internalCloseIsButtonTag:{type:Boolean,default:!0},onCheckedChange:Function}),A=S(`n-tag`),j=h({name:`Tag`,props:k,slots:Object,setup(r){let a=i(null),{mergedBorderedRef:o,mergedClsPrefixRef:s,inlineThemeDisabled:c,mergedRtlRef:f,mergedComponentPropsRef:p}=m(r),h=t(()=>r.size||p?.value?.Tag?.size||`medium`),_=y(`Tag`,`-tag`,O,E,r,s);d(A,{roundRef:n(r,`round`)});function b(){if(!r.disabled&&r.checkable){let{checked:e,onCheckedChange:t,onUpdateChecked:n,"onUpdate:checked":i}=r;n&&n(!e),i&&i(!e),t&&t(!e)}}function S(e){if(r.triggerClickOnClose||e.stopPropagation(),!r.disabled){let{onClose:t}=r;t&&v(t,e)}}let C={setTextContent(e){let{value:t}=a;t&&(t.textContent=e)}},w=x(`Tag`,f,s),T=t(()=>{let{type:e,color:{color:t,textColor:n}={}}=r,i=h.value,{common:{cubicBezierEaseInOut:a},self:{padding:s,closeMargin:c,borderRadius:d,opacityDisabled:f,textColorCheckable:p,textColorHoverCheckable:m,textColorPressedCheckable:g,textColorChecked:v,colorCheckable:y,colorHoverCheckable:b,colorPressedCheckable:x,colorChecked:S,colorCheckedHover:C,colorCheckedPressed:w,closeBorderRadius:T,fontWeightStrong:E,[u(`colorBordered`,e)]:D,[u(`closeSize`,i)]:O,[u(`closeIconSize`,i)]:k,[u(`fontSize`,i)]:A,[u(`height`,i)]:j,[u(`color`,e)]:M,[u(`textColor`,e)]:N,[u(`border`,e)]:P,[u(`closeIconColor`,e)]:F,[u(`closeIconColorHover`,e)]:I,[u(`closeIconColorPressed`,e)]:L,[u(`closeColorHover`,e)]:R,[u(`closeColorPressed`,e)]:z}}=_.value,B=l(c);return{"--n-font-weight-strong":E,"--n-avatar-size-override":`calc(${j} - 8px)`,"--n-bezier":a,"--n-border-radius":d,"--n-border":P,"--n-close-icon-size":k,"--n-close-color-pressed":z,"--n-close-color-hover":R,"--n-close-border-radius":T,"--n-close-icon-color":F,"--n-close-icon-color-hover":I,"--n-close-icon-color-pressed":L,"--n-close-icon-color-disabled":F,"--n-close-margin-top":B.top,"--n-close-margin-right":B.right,"--n-close-margin-bottom":B.bottom,"--n-close-margin-left":B.left,"--n-close-size":O,"--n-color":t||(o.value?D:M),"--n-color-checkable":y,"--n-color-checked":S,"--n-color-checked-hover":C,"--n-color-checked-pressed":w,"--n-color-hover-checkable":b,"--n-color-pressed-checkable":x,"--n-font-size":A,"--n-height":j,"--n-opacity-disabled":f,"--n-padding":s,"--n-text-color":n||N,"--n-text-color-checkable":p,"--n-text-color-checked":v,"--n-text-color-hover-checkable":m,"--n-text-color-pressed-checkable":g}}),D=c?e(`tag`,t(()=>{let e=``,{type:t,color:{color:n,textColor:i}={}}=r;return e+=t[0],e+=h.value[0],n&&(e+=`a${g(n)}`),i&&(e+=`b${g(i)}`),o.value&&(e+=`c`),e}),T,r):void 0;return Object.assign(Object.assign({},C),{rtlEnabled:w,mergedClsPrefix:s,contentRef:a,mergedBordered:o,handleClick:b,handleCloseClick:S,cssVars:c?void 0:T,themeClass:D?.themeClass,onRender:D?.onRender})},render(){var e;let{mergedClsPrefix:t,rtlEnabled:n,closable:r,color:{borderColor:i}={},round:a,onRender:o,$slots:s}=this;o?.();let c=p(s.avatar,e=>e&&f(`div`,{class:`${t}-tag__avatar`},e)),l=p(s.icon,e=>e&&f(`div`,{class:`${t}-tag__icon`},e));return f(`div`,{class:[`${t}-tag`,this.themeClass,{[`${t}-tag--rtl`]:n,[`${t}-tag--strong`]:this.strong,[`${t}-tag--disabled`]:this.disabled,[`${t}-tag--checkable`]:this.checkable,[`${t}-tag--checked`]:this.checkable&&this.checked,[`${t}-tag--round`]:a,[`${t}-tag--avatar`]:c,[`${t}-tag--icon`]:l,[`${t}-tag--closable`]:r}],style:this.cssVars,onClick:this.handleClick,onMouseenter:this.onMouseenter,onMouseleave:this.onMouseleave},l||c,f(`span`,{class:`${t}-tag__content`,ref:`contentRef`},(e=this.$slots).default?.call(e)),!this.checkable&&r?f(C,{clsPrefix:t,class:`${t}-tag__close`,disabled:this.disabled,onClick:this.handleCloseClick,focusable:this.internalCloseFocusable,round:a,isButtonTag:this.internalCloseIsButtonTag,absolute:!0}):null,!this.checkable&&this.mergedBordered?f(`div`,{class:`${t}-tag__border`,style:{borderColor:i}}):null)}});export{j as t};