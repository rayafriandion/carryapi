import{$ as e,$t as t,Bn as n,Dt as r,Ft as i,In as a,It as o,Lt as s,Mt as c,Nt as l,Q as u,Rt as d,at as f,cn as p,ct as m,et as h,f as g,h as _,kt as v,m as y,on as b,u as x,ut as S,v as C,wt as w}from"./http-BFtIegGs.js";import{n as T}from"./_plugin-vue_export-helper-CEWy_VHC.js";var E={buttonHeightSmall:`14px`,buttonHeightMedium:`18px`,buttonHeightLarge:`22px`,buttonWidthSmall:`14px`,buttonWidthMedium:`18px`,buttonWidthLarge:`22px`,buttonWidthPressedSmall:`20px`,buttonWidthPressedMedium:`24px`,buttonWidthPressedLarge:`28px`,railHeightSmall:`18px`,railHeightMedium:`22px`,railHeightLarge:`26px`,railWidthSmall:`32px`,railWidthMedium:`40px`,railWidthLarge:`48px`};function D(e){let{primaryColor:t,opacityDisabled:n,borderRadius:r,textColor3:i}=e;return Object.assign(Object.assign({},E),{iconColor:i,textColor:`white`,loadingColor:t,opacityDisabled:n,railColor:`rgba(0, 0, 0, .14)`,railColorActive:t,buttonBoxShadow:`0 1px 4px 0 rgba(0, 0, 0, 0.3), inset 0 0 1px 0 rgba(0, 0, 0, 0.05)`,buttonColor:`#FFF`,railBorderRadiusSmall:r,railBorderRadiusMedium:r,railBorderRadiusLarge:r,buttonBorderRadiusSmall:r,buttonBorderRadiusMedium:r,buttonBorderRadiusLarge:r,boxShadowFocus:`0 0 0 2px ${w(t,{alpha:.2})}`})}var O={name:`Switch`,common:x,self:D},k=l(`switch`,`
 height: var(--n-height);
 min-width: var(--n-width);
 vertical-align: middle;
 user-select: none;
 -webkit-user-select: none;
 display: inline-flex;
 outline: none;
 justify-content: center;
 align-items: center;
`,[i(`children-placeholder`,`
 height: var(--n-rail-height);
 display: flex;
 flex-direction: column;
 overflow: hidden;
 pointer-events: none;
 visibility: hidden;
 `),i(`rail-placeholder`,`
 display: flex;
 flex-wrap: none;
 `),i(`button-placeholder`,`
 width: calc(1.75 * var(--n-rail-height));
 height: var(--n-rail-height);
 `),l(`base-loading`,`
 position: absolute;
 top: 50%;
 left: 50%;
 transform: translateX(-50%) translateY(-50%);
 font-size: calc(var(--n-button-width) - 4px);
 color: var(--n-loading-color);
 transition: color .3s var(--n-bezier);
 `,[y({left:`50%`,top:`50%`,originalTransform:`translateX(-50%) translateY(-50%)`})]),i(`checked, unchecked`,`
 transition: color .3s var(--n-bezier);
 color: var(--n-text-color);
 box-sizing: border-box;
 position: absolute;
 white-space: nowrap;
 top: 0;
 bottom: 0;
 display: flex;
 align-items: center;
 line-height: 1;
 `),i(`checked`,`
 right: 0;
 padding-right: calc(1.25 * var(--n-rail-height) - var(--n-offset));
 `),i(`unchecked`,`
 left: 0;
 justify-content: flex-end;
 padding-left: calc(1.25 * var(--n-rail-height) - var(--n-offset));
 `),c(`&:focus`,[i(`rail`,`
 box-shadow: var(--n-box-shadow-focus);
 `)]),o(`round`,[i(`rail`,`border-radius: calc(var(--n-rail-height) / 2);`,[i(`button`,`border-radius: calc(var(--n-button-height) / 2);`)])]),s(`disabled`,[s(`icon`,[o(`rubber-band`,[o(`pressed`,[i(`rail`,[i(`button`,`max-width: var(--n-button-width-pressed);`)])]),i(`rail`,[c(`&:active`,[i(`button`,`max-width: var(--n-button-width-pressed);`)])]),o(`active`,[o(`pressed`,[i(`rail`,[i(`button`,`left: calc(100% - var(--n-offset) - var(--n-button-width-pressed));`)])]),i(`rail`,[c(`&:active`,[i(`button`,`left: calc(100% - var(--n-offset) - var(--n-button-width-pressed));`)])])])])])]),o(`active`,[i(`rail`,[i(`button`,`left: calc(100% - var(--n-button-width) - var(--n-offset))`)])]),i(`rail`,`
 overflow: hidden;
 height: var(--n-rail-height);
 min-width: var(--n-rail-width);
 border-radius: var(--n-rail-border-radius);
 cursor: pointer;
 position: relative;
 transition:
 opacity .3s var(--n-bezier),
 background .3s var(--n-bezier),
 box-shadow .3s var(--n-bezier);
 background-color: var(--n-rail-color);
 `,[i(`button-icon`,`
 color: var(--n-icon-color);
 transition: color .3s var(--n-bezier);
 font-size: calc(var(--n-button-height) - 4px);
 position: absolute;
 left: 0;
 right: 0;
 top: 0;
 bottom: 0;
 display: flex;
 justify-content: center;
 align-items: center;
 line-height: 1;
 `,[y()]),i(`button`,`
 align-items: center; 
 top: var(--n-offset);
 left: var(--n-offset);
 height: var(--n-button-height);
 width: var(--n-button-width-pressed);
 max-width: var(--n-button-width);
 border-radius: var(--n-button-border-radius);
 background-color: var(--n-button-color);
 box-shadow: var(--n-button-box-shadow);
 box-sizing: border-box;
 cursor: inherit;
 content: "";
 position: absolute;
 transition:
 background-color .3s var(--n-bezier),
 left .3s var(--n-bezier),
 opacity .3s var(--n-bezier),
 max-width .3s var(--n-bezier),
 box-shadow .3s var(--n-bezier);
 `)]),o(`active`,[i(`rail`,`background-color: var(--n-rail-color-active);`)]),o(`loading`,[i(`rail`,`
 cursor: wait;
 `)]),o(`disabled`,[i(`rail`,`
 cursor: not-allowed;
 opacity: .5;
 `)])]),A=Object.assign(Object.assign({},C.props),{size:String,value:{type:[String,Number,Boolean],default:void 0},loading:Boolean,defaultValue:{type:[String,Number,Boolean],default:!1},disabled:{type:Boolean,default:void 0},round:{type:Boolean,default:!0},"onUpdate:value":[Function,Array],onUpdateValue:[Function,Array],checkedValue:{type:[String,Number,Boolean],default:!0},uncheckedValue:{type:[String,Number,Boolean],default:!1},railStyle:Function,rubberBand:{type:Boolean,default:!0},spinProps:Object,onChange:[Function,Array]}),j,M=b({name:`Switch`,props:A,slots:Object,setup(i){j===void 0&&(j=typeof CSS<`u`?CSS.supports!==void 0&&CSS.supports(`width`,`max(1px)`):!0);let{mergedClsPrefixRef:o,inlineThemeDisabled:s,mergedComponentPropsRef:c}=h(i),l=C(`Switch`,`-switch`,k,O,i,o),f=u(i,{mergedSize(e){return i.size===void 0?e?e.mergedSize.value:c?.value?.Switch?.size||`medium`:i.size}}),{mergedSizeRef:p,mergedDisabledRef:m}=f,g=a(i.defaultValue),_=n(i,`value`),y=T(_,g),b=t(()=>y.value===i.checkedValue),x=a(!1),w=a(!1),E=t(()=>{let{railStyle:e}=i;if(e)return e({focused:w.value,checked:b.value})});function D(e){let{"onUpdate:value":t,onChange:n,onUpdateValue:r}=i,{nTriggerFormInput:a,nTriggerFormChange:o}=f;t&&S(t,e),r&&S(r,e),n&&S(n,e),g.value=e,a(),o()}function A(){let{nTriggerFormFocus:e}=f;e()}function M(){let{nTriggerFormBlur:e}=f;e()}function N(){i.loading||m.value||(y.value===i.checkedValue?D(i.uncheckedValue):D(i.checkedValue))}function P(){w.value=!0,A()}function F(){w.value=!1,M(),x.value=!1}function I(e){i.loading||m.value||e.key===` `&&(y.value===i.checkedValue?D(i.uncheckedValue):D(i.checkedValue),x.value=!1)}function L(e){i.loading||m.value||e.key===` `&&(e.preventDefault(),x.value=!0)}let R=t(()=>{let{value:e}=p,{self:{opacityDisabled:t,railColor:n,railColorActive:i,buttonBoxShadow:a,buttonColor:o,boxShadowFocus:s,loadingColor:c,textColor:u,iconColor:f,[d(`buttonHeight`,e)]:m,[d(`buttonWidth`,e)]:h,[d(`buttonWidthPressed`,e)]:g,[d(`railHeight`,e)]:_,[d(`railWidth`,e)]:y,[d(`railBorderRadius`,e)]:b,[d(`buttonBorderRadius`,e)]:x},common:{cubicBezierEaseInOut:S}}=l.value,C,w,T;return j?(C=`calc((${_} - ${m}) / 2)`,w=`max(${_}, ${m})`,T=`max(${y}, calc(${y} + ${m} - ${_}))`):(C=v((r(_)-r(m))/2),w=v(Math.max(r(_),r(m))),T=r(_)>r(m)?y:v(r(y)+r(m)-r(_))),{"--n-bezier":S,"--n-button-border-radius":x,"--n-button-box-shadow":a,"--n-button-color":o,"--n-button-width":h,"--n-button-width-pressed":g,"--n-button-height":m,"--n-height":w,"--n-offset":C,"--n-opacity-disabled":t,"--n-rail-border-radius":b,"--n-rail-color":n,"--n-rail-color-active":i,"--n-rail-height":_,"--n-rail-width":y,"--n-width":T,"--n-box-shadow-focus":s,"--n-loading-color":c,"--n-text-color":u,"--n-icon-color":f}}),z=s?e(`switch`,t(()=>p.value[0]),R,i):void 0;return{handleClick:N,handleBlur:F,handleFocus:P,handleKeyup:I,handleKeydown:L,mergedRailStyle:E,pressed:x,mergedClsPrefix:o,mergedValue:y,checked:b,mergedDisabled:m,cssVars:s?void 0:R,themeClass:z?.themeClass,onRender:z?.onRender}},render(){let{mergedClsPrefix:e,mergedDisabled:t,checked:n,mergedRailStyle:r,onRender:i,$slots:a}=this;i?.();let{checked:o,unchecked:s,icon:c,"checked-icon":l,"unchecked-icon":u}=a,d=!(f(c)&&f(l)&&f(u));return p(`div`,{role:`switch`,"aria-checked":n,class:[`${e}-switch`,this.themeClass,d&&`${e}-switch--icon`,n&&`${e}-switch--active`,t&&`${e}-switch--disabled`,this.round&&`${e}-switch--round`,this.loading&&`${e}-switch--loading`,this.pressed&&`${e}-switch--pressed`,this.rubberBand&&`${e}-switch--rubber-band`],tabindex:this.mergedDisabled?void 0:0,style:this.cssVars,onClick:this.handleClick,onFocus:this.handleFocus,onBlur:this.handleBlur,onKeyup:this.handleKeyup,onKeydown:this.handleKeydown},p(`div`,{class:`${e}-switch__rail`,"aria-hidden":`true`,style:r},m(o,t=>m(s,n=>t||n?p(`div`,{"aria-hidden":!0,class:`${e}-switch__children-placeholder`},p(`div`,{class:`${e}-switch__rail-placeholder`},p(`div`,{class:`${e}-switch__button-placeholder`}),t),p(`div`,{class:`${e}-switch__rail-placeholder`},p(`div`,{class:`${e}-switch__button-placeholder`}),n)):null)),p(`div`,{class:`${e}-switch__button`},m(c,t=>m(l,n=>m(u,r=>p(_,null,{default:()=>this.loading?p(g,Object.assign({key:`loading`,clsPrefix:e,strokeWidth:20},this.spinProps)):this.checked&&(n||t)?p(`div`,{class:`${e}-switch__button-icon`,key:n?`checked-icon`:`icon`},n||t):!this.checked&&(r||t)?p(`div`,{class:`${e}-switch__button-icon`,key:r?`unchecked-icon`:`icon`},r||t):null})))),m(o,t=>t&&p(`div`,{key:`checked`,class:`${e}-switch__checked`},t)),m(s,t=>t&&p(`div`,{key:`unchecked`,class:`${e}-switch__unchecked`},t)))))}});export{M as t};