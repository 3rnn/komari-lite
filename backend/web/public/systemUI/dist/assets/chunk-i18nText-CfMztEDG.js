import{x as h}from"./entry-index-D048Qa8x.js";/**
 * @license lucide-react v0.511.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */const l=[["path",{d:"m3 17 2 2 4-4",key:"1jhpwq"}],["path",{d:"m3 7 2 2 4-4",key:"1obspn"}],["path",{d:"M13 6h8",key:"15sg57"}],["path",{d:"M13 12h8",key:"h98zly"}],["path",{d:"M13 18h8",key:"oe0vm4"}]],m=h("list-checks",l);function u(t,c){if(t==null)return;if(typeof t=="string")return t;const n=Object.entries(t).filter(([,e])=>typeof e=="string"&&e.trim()!=="");if(n.length===0)return;const o=e=>e.trim().replace(/_/g,"-").toLowerCase(),i=n.find(([e])=>o(e)==="en")||n.find(([e])=>o(e).startsWith("en-"));if(i)return i[1];const r=o(c||""),a=r.split("-")[0],d=[r,a].filter(Boolean);for(const e of d){const s=n.find(([f])=>o(f)===e);if(s)return s[1]}return n[0][1]}export{m as L,u as r};
