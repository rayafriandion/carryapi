import{$ as e,$t as t,Bt as n,Ft as r,It as i,Lt as a,Mt as o,Nt as s,Rt as c,Tt as l,cn as u,et as d,on as f,u as p,v as m,zt as h}from"./http-QSbWvBrg.js";import{n as g,o as _}from"./render-BqsyCiMX.js";import{t as v}from"./use-compitable-BKENY7TC.js";import{t as y}from"./get-slot-6kXJmSMP.js";function b(e,t=`default`,n=[]){let{children:r}=e;if(typeof r==`object`&&r&&!Array.isArray(r)){let e=r[t];if(typeof e==`function`)return e()}return n}var x={thPaddingBorderedSmall:`8px 12px`,thPaddingBorderedMedium:`12px 16px`,thPaddingBorderedLarge:`16px 24px`,thPaddingSmall:`0`,thPaddingMedium:`0`,thPaddingLarge:`0`,tdPaddingBorderedSmall:`8px 12px`,tdPaddingBorderedMedium:`12px 16px`,tdPaddingBorderedLarge:`16px 24px`,tdPaddingSmall:`0 0 8px 0`,tdPaddingMedium:`0 0 12px 0`,tdPaddingLarge:`0 0 16px 0`};function S(e){let{tableHeaderColor:t,textColor2:n,textColor1:r,cardColor:i,modalColor:a,popoverColor:o,dividerColor:s,borderRadius:c,fontWeightStrong:u,lineHeight:d,fontSizeSmall:f,fontSizeMedium:p,fontSizeLarge:m}=e;return Object.assign(Object.assign({},x),{lineHeight:d,fontSizeSmall:f,fontSizeMedium:p,fontSizeLarge:m,titleTextColor:r,thColor:l(i,t),thColorModal:l(a,t),thColorPopover:l(o,t),thTextColor:r,thFontWeight:u,tdTextColor:n,tdColor:i,tdColorModal:a,tdColorPopover:o,borderColor:l(i,s),borderColorModal:l(a,s),borderColorPopover:l(o,s),borderRadius:c})}var C={name:`Descriptions`,common:p,self:S},w=o([s(`descriptions`,{fontSize:`var(--n-font-size)`},[s(`descriptions-separator`,`
 display: inline-block;
 margin: 0 8px 0 2px;
 `),s(`descriptions-table-wrapper`,[s(`descriptions-table`,[s(`descriptions-table-row`,[s(`descriptions-table-header`,{padding:`var(--n-th-padding)`}),s(`descriptions-table-content`,{padding:`var(--n-td-padding)`})])])]),a(`bordered`,[s(`descriptions-table-wrapper`,[s(`descriptions-table`,[s(`descriptions-table-row`,[o(`&:last-child`,[s(`descriptions-table-content`,{paddingBottom:0})])])])])]),i(`left-label-placement`,[s(`descriptions-table-content`,[o(`> *`,{verticalAlign:`top`})])]),i(`left-label-align`,[o(`th`,{textAlign:`left`})]),i(`center-label-align`,[o(`th`,{textAlign:`center`})]),i(`right-label-align`,[o(`th`,{textAlign:`right`})]),i(`bordered`,[s(`descriptions-table-wrapper`,`
 border-radius: var(--n-border-radius);
 overflow: hidden;
 background: var(--n-merged-td-color);
 border: 1px solid var(--n-merged-border-color);
 `,[s(`descriptions-table`,[s(`descriptions-table-row`,[o(`&:not(:last-child)`,[s(`descriptions-table-content`,{borderBottom:`1px solid var(--n-merged-border-color)`}),s(`descriptions-table-header`,{borderBottom:`1px solid var(--n-merged-border-color)`})]),s(`descriptions-table-header`,`
 font-weight: 400;
 background-clip: padding-box;
 background-color: var(--n-merged-th-color);
 `,[o(`&:not(:last-child)`,{borderRight:`1px solid var(--n-merged-border-color)`})]),s(`descriptions-table-content`,[o(`&:not(:last-child)`,{borderRight:`1px solid var(--n-merged-border-color)`})])])])])]),s(`descriptions-header`,`
 font-weight: var(--n-th-font-weight);
 font-size: 18px;
 transition: color .3s var(--n-bezier);
 line-height: var(--n-line-height);
 margin-bottom: 16px;
 color: var(--n-title-text-color);
 `),s(`descriptions-table-wrapper`,`
 transition:
 background-color .3s var(--n-bezier),
 border-color .3s var(--n-bezier);
 `,[s(`descriptions-table`,`
 width: 100%;
 border-collapse: separate;
 border-spacing: 0;
 box-sizing: border-box;
 `,[s(`descriptions-table-row`,`
 box-sizing: border-box;
 transition: border-color .3s var(--n-bezier);
 `,[s(`descriptions-table-header`,`
 font-weight: var(--n-th-font-weight);
 line-height: var(--n-line-height);
 display: table-cell;
 box-sizing: border-box;
 color: var(--n-th-text-color);
 transition:
 color .3s var(--n-bezier),
 background-color .3s var(--n-bezier),
 border-color .3s var(--n-bezier);
 `),s(`descriptions-table-content`,`
 vertical-align: top;
 line-height: var(--n-line-height);
 display: table-cell;
 box-sizing: border-box;
 color: var(--n-td-text-color);
 transition:
 color .3s var(--n-bezier),
 background-color .3s var(--n-bezier),
 border-color .3s var(--n-bezier);
 `,[r(`content`,`
 transition: color .3s var(--n-bezier);
 display: inline-block;
 color: var(--n-td-text-color);
 `)]),r(`label`,`
 font-weight: var(--n-th-font-weight);
 transition: color .3s var(--n-bezier);
 display: inline-block;
 margin-right: 14px;
 color: var(--n-th-text-color);
 `)])])])]),s(`descriptions-table-wrapper`,`
 --n-merged-th-color: var(--n-th-color);
 --n-merged-td-color: var(--n-td-color);
 --n-merged-border-color: var(--n-border-color);
 `),h(s(`descriptions-table-wrapper`,`
 --n-merged-th-color: var(--n-th-color-modal);
 --n-merged-td-color: var(--n-td-color-modal);
 --n-merged-border-color: var(--n-border-color-modal);
 `)),n(s(`descriptions-table-wrapper`,`
 --n-merged-th-color: var(--n-th-color-popover);
 --n-merged-td-color: var(--n-td-color-popover);
 --n-merged-border-color: var(--n-border-color-popover);
 `))]),T=`DESCRIPTION_ITEM_FLAG`;function E(e){return typeof e==`object`&&e&&!Array.isArray(e)?e.type&&e.type.DESCRIPTION_ITEM_FLAG:!1}var D=Object.assign(Object.assign({},m.props),{title:String,column:{type:Number,default:3},columns:Number,labelPlacement:{type:String,default:`top`},labelAlign:{type:String,default:`left`},separator:{type:String,default:`:`},size:String,bordered:Boolean,labelClass:String,labelStyle:[Object,String],contentClass:String,contentStyle:[Object,String]}),O=f({name:`Descriptions`,props:D,slots:Object,setup(n){let{mergedClsPrefixRef:r,inlineThemeDisabled:i,mergedComponentPropsRef:a}=d(n),o=t(()=>n.size||a?.value?.Descriptions?.size||`medium`),s=m(`Descriptions`,`-descriptions`,w,C,n,r),l=t(()=>{let{bordered:e}=n,t=o.value,{common:{cubicBezierEaseInOut:r},self:{titleTextColor:i,thColor:a,thColorModal:l,thColorPopover:u,thTextColor:d,thFontWeight:f,tdTextColor:p,tdColor:m,tdColorModal:h,tdColorPopover:g,borderColor:_,borderColorModal:v,borderColorPopover:y,borderRadius:b,lineHeight:x,[c(`fontSize`,t)]:S,[c(e?`thPaddingBordered`:`thPadding`,t)]:C,[c(e?`tdPaddingBordered`:`tdPadding`,t)]:w}}=s.value;return{"--n-title-text-color":i,"--n-th-padding":C,"--n-td-padding":w,"--n-font-size":S,"--n-bezier":r,"--n-th-font-weight":f,"--n-line-height":x,"--n-th-text-color":d,"--n-td-text-color":p,"--n-th-color":a,"--n-th-color-modal":l,"--n-th-color-popover":u,"--n-td-color":m,"--n-td-color-modal":h,"--n-td-color-popover":g,"--n-border-radius":b,"--n-border-color":_,"--n-border-color-modal":v,"--n-border-color-popover":y}}),u=i?e(`descriptions`,t(()=>{let e=``,{bordered:t}=n;return t&&(e+=`a`),e+=o.value[0],e}),l,n):void 0;return{mergedClsPrefix:r,cssVars:i?void 0:l,themeClass:u?.themeClass,onRender:u?.onRender,compitableColumn:v(n,[`columns`,`column`]),inlineThemeDisabled:i,mergedSize:o}},render(){let e=this.$slots.default,t=e?g(e()):[];t.length;let{contentClass:n,labelClass:r,compitableColumn:i,labelPlacement:a,labelAlign:o,mergedSize:s,bordered:c,title:l,cssVars:d,mergedClsPrefix:f,separator:p,onRender:m}=this;m?.();let h=t.filter(e=>E(e)),v=h.reduce((e,t,o)=>{let s=t.props||{},l=h.length-1===o,d=[`label`in s?s.label:b(t,`label`)],m=[b(t)],g=s.span||1,_=e.span;e.span+=g;let v=s.labelStyle||s[`label-style`]||this.labelStyle,y=s.contentStyle||s[`content-style`]||this.contentStyle;if(a===`left`)c?e.row.push(u(`th`,{class:[`${f}-descriptions-table-header`,r],colspan:1,style:v},d),u(`td`,{class:[`${f}-descriptions-table-content`,n],colspan:l?(i-_)*2+1:g*2-1,style:y},m)):e.row.push(u(`td`,{class:`${f}-descriptions-table-content`,colspan:l?(i-_)*2:g*2},u(`span`,{class:[`${f}-descriptions-table-content__label`,r],style:v},[...d,p&&u(`span`,{class:`${f}-descriptions-separator`},p)]),u(`span`,{class:[`${f}-descriptions-table-content__content`,n],style:y},m)));else{let t=l?(i-_)*2:g*2;e.row.push(u(`th`,{class:[`${f}-descriptions-table-header`,r],colspan:t,style:v},d)),e.secondRow.push(u(`td`,{class:[`${f}-descriptions-table-content`,n],colspan:t,style:y},m))}return(e.span>=i||l)&&(e.span=0,e.row.length&&(e.rows.push(e.row),e.row=[]),a!==`left`&&e.secondRow.length&&(e.rows.push(e.secondRow),e.secondRow=[])),e},{span:0,row:[],secondRow:[],rows:[]}).rows.map(e=>u(`tr`,{class:`${f}-descriptions-table-row`},e));return u(`div`,{style:d,class:[`${f}-descriptions`,this.themeClass,`${f}-descriptions--${a}-label-placement`,`${f}-descriptions--${o}-label-align`,`${f}-descriptions--${s}-size`,c&&`${f}-descriptions--bordered`]},l||this.$slots.header?u(`div`,{class:`${f}-descriptions-header`},l||y(this,`header`)):null,u(`div`,{class:`${f}-descriptions-table-wrapper`},u(`table`,{class:`${f}-descriptions-table`},u(`tbody`,null,a===`top`&&u(`tr`,{class:`${f}-descriptions-table-row`,style:{visibility:`collapse`}},_(i*2,u(`td`,null))),v))))}}),k={label:String,span:{type:Number,default:1},labelClass:String,labelStyle:[Object,String],contentClass:String,contentStyle:[Object,String]},A=f({name:`DescriptionsItem`,[T]:!0,props:k,slots:Object,render(){return null}});export{O as n,A as t};