import{w as h}from"./entry-index-Cht7QUEL.js";/**
 * @license lucide-react v0.511.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */const f=[["path",{d:"m3 17 2 2 4-4",key:"1jhpwq"}],["path",{d:"m3 7 2 2 4-4",key:"1obspn"}],["path",{d:"M13 6h8",key:"15sg57"}],["path",{d:"M13 12h8",key:"h98zly"}],["path",{d:"M13 18h8",key:"oe0vm4"}]],l=h("list-checks",f);function m(t,i){if(t==null)return;if(typeof t=="string")return t;const n=Object.entries(t).filter(([,e])=>typeof e=="string"&&e.trim()!=="");if(n.length===0)return;const o=e=>e.trim().replace(/_/g,"-").toLowerCase(),r=o(i||""),c=r.split("-")[0],a=[r,c,"en","en-us"].filter(Boolean);for(const e of a){const s=n.find(([d])=>o(d)===e);if(s)return s[1]}return n[0][1]}export{l as L,m as r};
