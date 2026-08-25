// @ts-nocheck -- generated JavaScript is type-checked before transformation.
// AUTO-GENERATED FILE. DO NOT EDIT DIRECTLY.
// Source: scripts/templates/dom-primitives.ts.tpl + partials/
// Action family: form
// Generator: scripts/build/generate-dom-primitives.js
// jscpd:ignore-start -- injected functions must be self-contained when Chrome serializes them.

import type { DOMPrimitiveOptions, DOMResult } from '../dom-types.js'

export function domPrimitiveForm(action: string, selector: string, options: DOMPrimitiveOptions): DOMResult | Promise<DOMResult>{function getShadowRoot(el2){return el2.shadowRoot??null}function querySelectorDeep(selector2,root=document){
const fast=root.querySelector(selector2);if(fast&&!isKaboomOwnedElement(fast))return fast;return querySelectorDeepWalk(selector2,root)}function querySelectorDeepWalk(selector2,root,depth=0){
if(depth>10)return null;const children="children"in root?root.children:root.body?.children||root.documentElement?.children;if(!children)return null;for(let i=0;i<
children.length;i++){const child=children[i];const shadow=getShadowRoot(child);if(shadow){const match=shadow.querySelector(selector2);if(match&&!isKaboomOwnedElement(
match))return match;const deep=querySelectorDeepWalk(selector2,shadow,depth+1);if(deep)return deep}if(child.children.length>0){const deep=querySelectorDeepWalk(
selector2,child,depth+1);if(deep)return deep}}return null}function querySelectorAllDeep(selector2,root=document,results=[],depth=0){if(depth>10)return results;const matches=Array.
from(root.querySelectorAll(selector2));for(const match of matches){if(!isKaboomOwnedElement(match)){results.push(match)}}const children="children"in root?root.children:
root.body?.children||root.documentElement?.children;if(!children)return results;for(let i=0;i<children.length;i++){const child=children[i];const shadow=getShadowRoot(
child);if(shadow){querySelectorAllDeep(selector2,shadow,results,depth+1)}}return results}function resolveDeepCombinator(selector2,root=document){const parts=selector2.
split(" >>> ");if(parts.length<=1)return null;let current=root;for(let i=0;i<parts.length;i++){const part=parts[i].trim();if(i<parts.length-1){const host=querySelectorDeep(
part,current);if(!host)return null;const shadow=getShadowRoot(host);if(!shadow)return null;current=shadow}else{return querySelectorDeep(part,current)}}return null}
function isKaboomOwnedElement(element){let node=element;while(node){if(node.getAttribute&&node.getAttribute("data-kaboom-owned")==="true")return true;node=node.
parentElement}return false}function isVisible(el2){if(isKaboomOwnedElement(el2))return false;if(!(el2 instanceof HTMLElement))return true;const style=getComputedStyle(
el2);if(style.visibility==="hidden"||style.display==="none")return false;if(el2.offsetParent===null&&style.position!=="fixed"&&style.position!=="sticky"){const rect=el2.
getBoundingClientRect();if(rect.width===0&&rect.height===0)return false}return true}function firstVisible(els){let fallback=null;for(const el2 of els){if(!fallback)
fallback=el2;if(isVisible(el2))return el2}return fallback}function resolveScopeRoot(rawScope){const scope=(rawScope||"").trim();if(!scope)return document;try{const matches=querySelectorAllDeep(
scope);if(matches.length===0)return null;return firstVisible(matches)||matches[0]||null}catch{return null}}const scopeRoot=resolveScopeRoot(options.scope_selector);
function parseScopeRect(raw){if(!raw||typeof raw!=="object")return null;const rect=raw;const x=Number(rect.x);const y=Number(rect.y);const width=Number(rect.width);
const height=Number(rect.height);if(![x,y,width,height].every(v=>Number.isFinite(v)))return null;if(width<=0||height<=0)return null;return{x,y,width,height}}const scopeRect=parseScopeRect(
options.scope_rect);if(options.scope_rect!==void 0&&!scopeRect){return{success:false,action,selector,error:"invalid_scope_rect",message:"scope_rect must include\
 finite x, y, width, and height > 0"}}function intersectsScopeRect(el2){if(!scopeRect)return true;const htmlEl=el2;if(!htmlEl||typeof htmlEl.getBoundingClientRect!==
"function")return false;const rect=htmlEl.getBoundingClientRect();const left=typeof rect.left==="number"?rect.left:typeof rect.x==="number"?rect.x:0;const top=typeof rect.
top==="number"?rect.top:typeof rect.y==="number"?rect.y:0;const right=typeof rect.right==="number"?rect.right:left+rect.width;const bottom=typeof rect.bottom===
"number"?rect.bottom:top+rect.height;const scopeRight=scopeRect.x+scopeRect.width;const scopeBottom=scopeRect.y+scopeRect.height;const overlapX=left<scopeRight&&
right>scopeRect.x;const overlapY=top<scopeBottom&&bottom>scopeRect.y;return overlapX&&overlapY}function filterByScopeRect(elements){if(!scopeRect)return elements;
return elements.filter(el2=>intersectsScopeRect(el2))}function getElementHandleStore(){const root=globalThis;if(root.__kaboomElementHandles){if(!root.__kaboomElementHandles.
selectorByID){root.__kaboomElementHandles.selectorByID=new Map}return root.__kaboomElementHandles}const created={byElement:new WeakMap,byID:new Map,selectorByID:new Map,
nextID:1};root.__kaboomElementHandles=created;return created}function getOrCreateElementID(el2){const store=getElementHandleStore();const existing=store.byElement.
get(el2);if(existing){store.byID.set(existing,el2);return existing}const elementID=`el_${(store.nextID++).toString(36)}`;store.byElement.set(el2,elementID);store.
byID.set(elementID,el2);return elementID}function resolveElementByID(rawElementID){const elementID=(rawElementID||"").trim();if(!elementID)return null;const store=getElementHandleStore();
const node=store.byID.get(elementID);if(node&&node.isConnected!==false)return node;const storedSelector=store.selectorByID.get(elementID);if(storedSelector){const reresolved=resolveElement(
storedSelector,document);if(reresolved&&reresolved.isConnected!==false){store.byElement.set(reresolved,elementID);store.byID.set(elementID,reresolved);return reresolved}}
if(node)store.byID.delete(elementID);store.selectorByID.delete(elementID);return null}function closestInteractiveTarget(parent){const interactive=parent.closest(
'a, button, [role="button"], [role="link"], label, summary');if(interactive)return interactive;if(typeof parent.querySelectorAll==="function"){const childCandidates=parent.
querySelectorAll('a[href], button, input:not([type="hidden"]), select, textarea, [role="button"], [role="link"]');for(let ci=0;ci<childCandidates.length;ci++){const child=childCandidates[ci];
if(isActionableVisible(child))return child}}return parent}function walkShadowScopes(root,visit){const children="children"in root?root.children:root.body?.children||
root.documentElement?.children;if(!children)return;for(let i=0;i<children.length;i++){const shadow=getShadowRoot(children[i]);if(shadow)visit(shadow)}}function resolveByTextAll(searchText,scope=document){
const results=[];const seen=new Set;function walkScope(root){const walker=document.createTreeWalker(root,NodeFilter.SHOW_TEXT);while(walker.nextNode()){const node=walker.
currentNode;if(node.textContent&&node.textContent.trim().includes(searchText)){const parent=node.parentElement;if(!parent)continue;const target=closestInteractiveTarget(
parent);if(isKaboomOwnedElement(target)||!isVisible(target))continue;if(!seen.has(target)){seen.add(target);results.push(target)}}}walkShadowScopes(root,walkScope)}
walkScope(scope);return results}function resolveByLabelAll(labelText,scope=document){const labels=querySelectorAllDeep("label",scope);const results=[];const seen=new Set;
const allowGlobalIdLookup=scope===document||scope===document.body||scope===document.documentElement;for(const label of labels){if(label.textContent&&label.textContent.
trim().includes(labelText)){const forAttr=label.getAttribute("for");if(forAttr){const local=querySelectorAllDeep(`#${CSS.escape(forAttr)}`,scope)[0];const target=local||
(allowGlobalIdLookup?document.getElementById(forAttr):null);if(target&&!seen.has(target)){seen.add(target);results.push(target)}}const nested=label.querySelector(
"input, select, textarea");if(nested&&!seen.has(nested)){seen.add(nested);results.push(nested)}if(!seen.has(label)){seen.add(label);results.push(label)}}}return results}
function resolveByAriaLabelAll(al,scope=document){const results=[];const seen=new Set;const exact=querySelectorAllDeep(`[aria-label="${CSS.escape(al)}"]`,scope);
for(const el2 of exact){if(!seen.has(el2)){seen.add(el2);results.push(el2)}}const all=querySelectorAllDeep("[aria-label]",scope);for(const el2 of all){const label=el2.
getAttribute("aria-label")||"";if(label.startsWith(al)&&!seen.has(el2)){seen.add(el2);results.push(el2)}}return results}function resolveByText(searchText,scope=document){
let fallback=null;function walkScope(root){const walker=document.createTreeWalker(root,NodeFilter.SHOW_TEXT);while(walker.nextNode()){const node=walker.currentNode;
if(node.textContent&&node.textContent.trim().includes(searchText)){const parent=node.parentElement;if(!parent)continue;const target=closestInteractiveTarget(parent);
if(isKaboomOwnedElement(target))continue;if(!fallback)fallback=target;if(isVisible(target))return target}}let shadowResult=null;walkShadowScopes(root,shadow=>{if(!shadowResult)
shadowResult=walkScope(shadow)});return shadowResult}return walkScope(scope)||fallback}function resolveByLabel(labelText,scope=document){const labels=querySelectorAllDeep(
"label",scope);const allowGlobalIdLookup=scope===document||scope===document.body||scope===document.documentElement;for(const label of labels){if(label.textContent&&
label.textContent.trim().includes(labelText)){const forAttr=label.getAttribute("for");if(forAttr){const local=querySelectorAllDeep(`#${CSS.escape(forAttr)}`,scope)[0];
if(local)return local;const target=allowGlobalIdLookup?document.getElementById(forAttr):null;if(target)return target}const nested=label.querySelector("input, se\
lect, textarea");if(nested)return nested;return label}}return null}function resolveByAriaLabel(al,scope=document){const exact=querySelectorAllDeep(`[aria-label=\
"${CSS.escape(al)}"]`,scope);if(exact.length>0)return firstVisible(exact);const all=querySelectorAllDeep("[aria-label]",scope);let fallback=null;for(const el2 of all){
const label=el2.getAttribute("aria-label")||"";if(label.startsWith(al)){if(!fallback)fallback=el2;if(isVisible(el2))return el2}}return fallback}function parseNthMatchSelector(sel){
const nthMatch=sel.match(/^(.*):nth-match\((\d+)\)$/);if(!nthMatch)return null;const base=nthMatch[1]||"";const n=Number.parseInt(nthMatch[2]||"0",10);if(!base||
Number.isNaN(n)||n<1)return null;return{base,n}}function resolveElements(sel,scope=document){if(!sel)return[];if(sel.includes(" >>> ")){const deep=resolveDeepCombinator(
sel,scope);return deep?[deep]:[]}const parsedNth=parseNthMatchSelector(sel);if(parsedNth){const matches=resolveElements(parsedNth.base,scope);const target=matches[parsedNth.
n-1];return target?[target]:[]}if(sel.startsWith("text="))return resolveByTextAll(sel.slice("text=".length),scope);if(sel.startsWith("role="))return querySelectorAllDeep(
`[role="${CSS.escape(sel.slice("role=".length))}"]`,scope);if(sel.startsWith("placeholder="))return querySelectorAllDeep(`[placeholder="${CSS.escape(sel.slice("\
placeholder=".length))}"]`,scope);if(sel.startsWith("label="))return resolveByLabelAll(sel.slice("label=".length),scope);if(sel.startsWith("aria-label="))return resolveByAriaLabelAll(
sel.slice("aria-label=".length),scope);try{return querySelectorAllDeep(sel,scope)}catch{return[]}}function resolveElement(sel,scope=document){if(!sel)return null;
if(sel.includes(" >>> "))return resolveDeepCombinator(sel,scope);const parsedNth=parseNthMatchSelector(sel);if(parsedNth){const matches=resolveElements(parsedNth.
base,scope);return matches[parsedNth.n-1]||null}if(sel.startsWith("text="))return resolveByText(sel.slice("text=".length),scope);if(sel.startsWith("role="))return firstVisible(
querySelectorAllDeep(`[role="${CSS.escape(sel.slice("role=".length))}"]`,scope));if(sel.startsWith("placeholder="))return firstVisible(querySelectorAllDeep(`[pl\
aceholder="${CSS.escape(sel.slice("placeholder=".length))}"]`,scope));if(sel.startsWith("label="))return resolveByLabel(sel.slice("label=".length),scope);if(sel.
startsWith("aria-label="))return resolveByAriaLabel(sel.slice("aria-label=".length),scope);return querySelectorDeep(sel,scope)}function buildUniqueSelector(el2,htmlEl,fallbackSelector){
if(el2.id)return`#${CSS.escape(el2.id)}`;if(el2 instanceof HTMLInputElement&&el2.name)return`input[name="${CSS.escape(el2.name)}"]`;const ariaLabel=el2.getAttribute(
"aria-label");if(ariaLabel)return`[aria-label="${CSS.escape(ariaLabel)}"]`;const placeholder=el2.getAttribute("placeholder");if(placeholder)return`[placeholder=\
"${CSS.escape(placeholder)}"]`;const text=(htmlEl.textContent||"").trim().slice(0,40);if(text)return`text=${text}`;return fallbackSelector}function buildShadowSelector(el2){
const rootNode=el2.getRootNode();if(!(rootNode instanceof ShadowRoot))return null;const parts=[];let node=el2;let root=rootNode;while(root instanceof ShadowRoot){
const inner=buildUniqueSelector(node,node,node.tagName.toLowerCase());parts.unshift(inner);node=root.host;root=node.getRootNode()}const hostSelector=buildUniqueSelector(
node,node,node.tagName.toLowerCase());parts.unshift(hostSelector);return parts.join(" >>> ")}function classifyInput(el2){const inputType=el2.type||"text";if(inputType===
"submit"||inputType==="button"||inputType==="reset")return"button";if(inputType==="checkbox"||inputType==="radio")return"checkbox";return"input"}function classifyElement(el2){
const tag=el2.tagName.toLowerCase();if(tag==="a")return"link";if(tag==="button"||el2.getAttribute("role")==="button")return"button";if(tag==="input")return classifyInput(
el2);if(tag==="select")return"select";if(tag==="textarea")return"textarea";const roleClass={link:"link",tab:"tab",menuitem:"menuitem"};const byRole=roleClass[el2.
getAttribute("role")||""];if(byRole)return byRole;if(el2.getAttribute("contenteditable")==="true")return"textarea";return"interactive"}function isVisibleElement(el2){
const htmlEl=el2;if(!htmlEl||typeof htmlEl.getBoundingClientRect!=="function")return true;const rect=htmlEl.getBoundingClientRect();return rect.width>0&&rect.height>
0&&htmlEl.offsetParent!==null}function extractElementLabel(el2){const htmlEl=el2;return el2.getAttribute("aria-label")||el2.getAttribute("title")||el2.getAttribute(
"placeholder")||(htmlEl?.textContent||"").trim().slice(0,80)||el2.tagName.toLowerCase()}function chooseBestScopeMatch(matches){if(matches.length===1)return matches[0];
const submitVerb=/(post|share|publish|send|submit|save|done|continue|next|create|apply)/i;let best=matches[0];let bestScore=-1;for(const candidate of matches){const textboxes=querySelectorAllDeep(
'[role="textbox"], textarea, [contenteditable="true"]',candidate);const visibleTextboxes=textboxes.filter(isVisibleElement).length;const buttonCandidates=querySelectorAllDeep(
'button, [role="button"], input[type="submit"]',candidate);let visibleButtons=0;let submitLikeButtons=0;for(const btn of buttonCandidates){if(!isVisibleElement(
btn))continue;visibleButtons++;if(submitVerb.test(extractElementLabel(btn))){submitLikeButtons++}}const interactiveCandidates=querySelectorAllDeep('a[href], but\
ton, input, select, textarea, [role="button"], [role="link"], [role="tab"], [role="menuitem"], [contenteditable="true"]',candidate);const visibleInteractive=interactiveCandidates.
filter(isVisibleElement).length;const hiddenInteractive=Math.max(0,interactiveCandidates.length-visibleInteractive);const rect=candidate.getBoundingClientRect?.();
const areaScore2=rect&&rect.width>0&&rect.height>0?Math.min(20,Math.round(rect.width*rect.height/5e4)):0;const score=visibleTextboxes*1e3+submitLikeButtons*250+
visibleButtons*10+visibleInteractive-hiddenInteractive+areaScore2;if(score>bestScore){bestScore=score;best=candidate}}return best}function isOverlaySizedHighZIndexElement(el2){
if(!(el2 instanceof HTMLElement))return false;const style=getComputedStyle(el2);const zIndex=Number.parseInt(style.zIndex||"",10);if(Number.isNaN(zIndex)||zIndex<
1e3)return false;const position=style.position||"";if(position!=="fixed"&&position!=="absolute")return false;const rect=el2.getBoundingClientRect();if(rect.width<
100||rect.height<100)return false;if(style.display==="none"||style.visibility==="hidden"||style.opacity==="0")return false;return true}function findTopmostOverlay(){
const dialogSelectors=['[role="dialog"]','[role="alertdialog"]','[aria-modal="true"]',"dialog[open]",".modal.show",".modal.in",".modal.is-active",'.modal[style*\
="display: block"]',".overlay",".popup",".lightbox","[data-modal]","[data-overlay]","[data-dialog]"];const candidates=[];for(const dialogSelector of dialogSelectors){
candidates.push(...querySelectorAllDeep(dialogSelector))}const allElements=document.querySelectorAll("*");for(let i=0;i<allElements.length;i++){const el2=allElements[i];
if(isOverlaySizedHighZIndexElement(el2))candidates.push(el2)}const unique=uniqueElements(candidates).filter(isActionableVisible);if(unique.length===0)return null;
const ranked=unique.map((candidate,index)=>({element:candidate,score:elementZIndexScore(candidate)*1e3+areaScore(candidate,200)+index}));ranked.sort((a,b)=>b.score-
a.score);return ranked[0]?.element||null}function describeOverlay(el2){const tag=el2.tagName.toLowerCase();const role=el2.getAttribute("role")||"";const ariaModal=el2.
getAttribute("aria-modal")||"";let overlayType="unknown";if(tag==="dialog")overlayType="dialog";else if(role==="dialog"||role==="alertdialog")overlayType=role;else if(ariaModal===
"true")overlayType="modal";else overlayType="overlay";const overlaySelector=(()=>{if(el2.id)return`#${el2.id}`;if(role)return`${tag}[role="${role}"]`;const className=el2.
className;if(typeof className==="string"&&className.trim())return`${tag}.${className.trim().split(/\s+/)[0]}`;return tag})();const textPreview=(el2.textContent||
"").trim().slice(0,120);return{overlay_type:overlayType,overlay_selector:overlaySelector,overlay_text_preview:textPreview}}function domError(error,message){return{
success:false,action,selector,error,message}}function matchedTarget(node){const htmlEl=node;const textPreview=(htmlEl.textContent||"").trim().slice(0,80);const classList=typeof htmlEl.
className==="string"&&htmlEl.className?htmlEl.className.split(/\s+/).filter(Boolean).slice(0,5):void 0;return{tag:node.tagName.toLowerCase(),role:node.getAttribute(
"role")||void 0,aria_label:node.getAttribute("aria-label")||void 0,text_preview:textPreview||void 0,classes:classList&&classList.length>0?classList:void 0,selector,
element_id:getOrCreateElementID(node),bbox:extractBoundingBox(node),scope_selector_used:resolvedScopeSelector,...scopeRect?{scope_rect_used:scopeRect}:{}}}function viewportSize(){
const height=typeof window!=="undefined"&&typeof window.innerHeight==="number"?window.innerHeight:typeof document!=="undefined"&&document.documentElement?Number(
document.documentElement.clientHeight||0):0;const width=typeof window!=="undefined"&&typeof window.innerWidth==="number"?window.innerWidth:typeof document!=="un\
defined"&&document.documentElement?Number(document.documentElement.clientWidth||0):0;return{width,height}}function rectIntersectsViewport(rect,viewWidth,viewHeight){
const left=typeof rect.left==="number"?rect.left:typeof rect.x==="number"?rect.x:0;const top=typeof rect.top==="number"?rect.top:typeof rect.y==="number"?rect.y:
0;const right=typeof rect.right==="number"?rect.right:left+rect.width;const bottom=typeof rect.bottom==="number"?rect.bottom:top+rect.height;const intersectsX=viewWidth<=
0||right>0&&left<viewWidth;const intersectsY=viewHeight<=0||bottom>0&&top<viewHeight;return intersectsX&&intersectsY}function isActionableVisible(el2){if(!(el2 instanceof
HTMLElement))return true;const rect=typeof el2.getBoundingClientRect==="function"?el2.getBoundingClientRect():{width:0,height:0};if(!(rect.width>0&&rect.height>
0))return false;if(el2.offsetParent===null){const style=typeof getComputedStyle==="function"?getComputedStyle(el2):null;const position=style?.position||"";if(position!==
"fixed"&&position!=="sticky")return false}const{width:viewWidth,height:viewHeight}=viewportSize();return rectIntersectsViewport(rect,viewWidth,viewHeight)}function extractBoundingBox(el2){
if(!(el2 instanceof HTMLElement)||typeof el2.getBoundingClientRect!=="function"){return{x:0,y:0,width:0,height:0}}const rect=el2.getBoundingClientRect();const x=typeof rect.
left==="number"?rect.left:typeof rect.x==="number"?rect.x:0;const y=typeof rect.top==="number"?rect.top:typeof rect.y==="number"?rect.y:0;const width=Number.isFinite(
rect.width)?rect.width:0;const height=Number.isFinite(rect.height)?rect.height:0;return{x:Math.round(x),y:Math.round(y),width:Math.round(width),height:Math.round(
height)}}function summarizeCandidates(matches){return matches.slice(0,8).map(candidate=>{const htmlEl=candidate;const fallback=candidate.tagName.toLowerCase();return{
tag:fallback,role:candidate.getAttribute("role")||void 0,aria_label:candidate.getAttribute("aria-label")||void 0,text_preview:(htmlEl.textContent||"").trim().slice(
0,80)||void 0,selector:buildUniqueSelector(candidate,htmlEl,fallback),element_id:getOrCreateElementID(candidate),bbox:extractBoundingBox(candidate),visible:isActionableVisible(
candidate)}})}function uniqueElements(elements){const out=[];const seen=new Set;for(const element of elements){if(seen.has(element))continue;seen.add(element);out.
push(element)}return out}function elementZIndexScore(el2){if(!(el2 instanceof HTMLElement))return 0;const style=getComputedStyle(el2);const raw=style.zIndex||"";
const parsed=Number.parseInt(raw,10);if(Number.isNaN(parsed))return 0;return parsed}function areaScore(el2,max){if(!(el2 instanceof HTMLElement)||typeof el2.getBoundingClientRect!==
"function")return 0;const rect=el2.getBoundingClientRect();if(rect.width<=0||rect.height<=0)return 0;return Math.min(max,Math.round(rect.width*rect.height/1e4))}
function collectDialogs(){const selectors=['[role="dialog"]','[aria-modal="true"]',"dialog[open]"];const dialogs=[];for(const dialogSelector of selectors){dialogs.
push(...querySelectorAllDeep(dialogSelector))}return uniqueElements(dialogs).filter(isActionableVisible)}function pickTopDialog(dialogs){if(dialogs.length===0)return null;
const ranked=dialogs.map((dialog,index)=>({element:dialog,score:elementZIndexScore(dialog)*1e3+areaScore(dialog,200)+index})).sort((a,b)=>b.score-a.score);return ranked[0]?.
element||null}function selectorRankingLabel(selectorText){if(selectorText.startsWith("text="))return selectorText.slice(5);if(selectorText.startsWith("aria-labe\
l="))return selectorText.slice(11);if(selectorText.startsWith("label="))return selectorText.slice(6);if(selectorText.startsWith("placeholder="))return selectorText.
slice(12);return""}function clickActionScore(el2,tag,role){const isButtonLike=tag==="button"||role==="button"||tag==="input"&&(el2.type==="submit"||el2.type==="\
button");if(isButtonLike)return 100;if(tag==="a"||role==="link")return 40;return 0}function typeActionScore(el2,tag,role){const isFieldLike=tag==="input"||tag===
"textarea"||tag==="select"||el2.getAttribute("contenteditable")==="true"||role==="textbox";if(isFieldLike)return 100;if(tag==="button"||role==="button")return 10;
return 0}function elementTypeMatchScore(el2,tag,role,action2){const clickLikeActions=new Set(["click","key_press","focus","scroll_to","set_attribute","paste"]);
const typeLikeActions=new Set(["type","select","check"]);if(clickLikeActions.has(action2))return clickActionScore(el2,tag,role);if(typeLikeActions.has(action2))
return typeActionScore(el2,tag,role);return 0}function textMatchScore(el2,selectorLabel){if(!selectorLabel)return 0;const trimmedLabel=extractElementLabel(el2).
trim();if(trimmedLabel===selectorLabel){return 80}if(trimmedLabel.startsWith(selectorLabel)&&trimmedLabel.length<=selectorLabel.length+5){return 60}return 0}function primaryButtonScore(el2,tag,role){
if(tag!=="button"&&role!=="button")return 0;const htmlEl=el2;const cls=(typeof htmlEl.className==="string"?htmlEl.className:"").toLowerCase();const type=el2.getAttribute(
"type")||"";if(type==="submit")return 60;if(/\bprimary\b|\bbtn-primary\b|\bcta\b/.test(cls))return 60;const style=typeof getComputedStyle==="function"?getComputedStyle(
htmlEl):null;if(!style)return 0;const bg=style.backgroundColor||"";if(bg&&!/transparent|rgba\(0,\s*0,\s*0,\s*0\)|rgb\(255,\s*255,\s*255\)|rgb\(2[45]\d,\s*2[45]\d,\s*2[45]\d\)/.
test(bg)){return 30}return 0}function rankAmbiguousCandidates(candidates,action2,selectorText){const dialogs=collectDialogs();const topDialog=dialogs.length>0?pickTopDialog(
dialogs):null;const selectorLabel=selectorRankingLabel(selectorText);const scored=candidates.map(el2=>{const tag=el2.tagName.toLowerCase();const role=el2.getAttribute(
"role")||"";let score=0;if(topDialog&&typeof topDialog.contains==="function"&&topDialog.contains(el2)){score+=200}score+=elementTypeMatchScore(el2,tag,role,action2);
score+=textMatchScore(el2,selectorLabel);score+=primaryButtonScore(el2,tag,role);score+=Math.min(50,Math.max(0,elementZIndexScore(el2)));score+=areaScore(el2,30);
return{element:el2,score}});scored.sort((a,b)=>b.score-a.score);const topScore=scored[0]?.score??0;const secondScore=scored[1]?.score??0;const gap=topScore-secondScore;
const winner=gap>=50?scored[0]?.element??null:null;return{winner,gap,ranked:scored}}const requestedScope=(options.scope_selector||"").trim();const activeScope=scopeRoot||
document;const scopeSelectorUsed=requestedScope||void 0;const scopeRectUsed=scopeRect||void 0;function resolveIntentShortcut(){if(action==="wait_for_text"||action===
"wait_for_absent"){return{element:document.body,match_count:1,match_strategy:action}}if(action==="key_press"&&!selector&&!options.element_id){const target=document.
activeElement||document.body;if(target){return{element:target,match_count:1,match_strategy:"active_element_fallback"}}}return null}function resolveElementIDTarget(requestedElementID,requestedScope2,scopeSelectorUsed2){
const resolvedByID=resolveElementByID(requestedElementID);if(!resolvedByID){return{error:domError("stale_element_id",`Element handle is stale or unknown: ${requestedElementID}\
. Call list_interactive again.`)}}if(activeScope!==document&&typeof activeScope.contains==="function"){const contains=activeScope.contains(resolvedByID);if(!contains){
return{error:domError("element_id_scope_mismatch",`Element handle does not belong to scope: ${requestedScope2||"<none>"}`)}}}if(scopeRect&&!intersectsScopeRect(
resolvedByID)){return{error:domError("element_id_scope_mismatch",`Element handle does not intersect scope_rect (${scopeRect.x}, ${scopeRect.y}, ${scopeRect.width}\
, ${scopeRect.height}).`)}}return{element:resolvedByID,match_count:1,match_strategy:"element_id",scope_selector_used:scopeSelectorUsed2}}function resolveNthTarget(){
const nthParam=options.nth;if(nthParam===void 0||nthParam===null)return null;const nth=Number(nthParam);if(!Number.isInteger(nth)){return{error:domError("invali\
d_nth",`nth must be an integer, got: ${nthParam}`)}}const allMatches=resolveElements(selector,activeScope);const uniqueAll=uniqueElements(allMatches);const rectFiltered=filterByScopeRect(
uniqueAll);const visibleFiltered=rectFiltered.filter(isActionableVisible);const candidates=visibleFiltered.length>0?visibleFiltered:rectFiltered;if(candidates.length===
0){return{error:domError("element_not_found",`No element matches selector: ${selector}`)}}const resolvedIndex=nth<0?candidates.length+nth:nth;if(resolvedIndex<0||
resolvedIndex>=candidates.length){return{error:domError("nth_out_of_range",`nth=${nth} is out of range \u2014 selector matched ${candidates.length} element(s). \
Use nth 0..${candidates.length-1} or -1..-${candidates.length}.`)}}return{element:candidates[resolvedIndex],match_count:candidates.length,match_strategy:"nth_pa\
ram",scope_selector_used:scopeSelectorUsed}}function selectorStrategy(){if(selector.includes(":nth-match("))return"nth_match_selector";if(scopeRectUsed)return"r\
ect_selector";if(requestedScope)return"scoped_selector";return"selector"}function buildAmbiguousInfo(){const allMatches=selector.startsWith("text=")?resolveElements(
selector,activeScope):null;if(!allMatches||allMatches.length<=1)return void 0;const uniqueAll=uniqueElements(allMatches);if(uniqueAll.length<=1)return void 0;return{
total_count:uniqueAll.length,warning:`Selector "${selector}" matched ${uniqueAll.length} elements. First match was used. Use nth, :nth-match(N), or scope_select\
or to disambiguate.`,candidates:uniqueAll.slice(0,5).map(c=>({tag:c.tagName.toLowerCase(),element_id:getOrCreateElementID(c),text_preview:(c.textContent||"").trim().
slice(0,60)||void 0}))}}function resolveUnambiguousTarget(){const ambiguousInfo=buildAmbiguousInfo();const direct=resolveElement(selector,activeScope);if(direct&&
intersectsScopeRect(direct)){return{element:direct,match_count:1,match_strategy:selectorStrategy(),scope_selector_used:scopeSelectorUsed,...ambiguousInfo?{ambiguous_matches:ambiguousInfo}:
{}}}const scopedMatches=filterByScopeRect(uniqueElements(resolveElements(selector,activeScope)));const found=(()=>{if(scopedMatches.length===0)return null;const visible=scopedMatches.
filter(isActionableVisible);return visible[0]||scopedMatches[0]||null})();if(!found)return{error:domError("element_not_found",`No element matches selector: ${selector}`)};
return{element:found,match_count:1,match_strategy:scopeRect?"rect_selector":requestedScope?"scoped_selector":"selector",scope_selector_used:scopeSelectorUsed,...ambiguousInfo?
{ambiguous_matches:ambiguousInfo}:{}}}function viableCandidates(uniqueMatches){const rectScopedMatches=filterByScopeRect(uniqueMatches);if(rectScopedMatches.length===
0)return rectScopedMatches;const visible=rectScopedMatches.filter(isActionableVisible);return visible.length>0?visible:rectScopedMatches}function resolveRankedTarget(){
const rawMatches=resolveElements(selector,activeScope);const uniqueMatches=[];const seen=new Set;for(const match of rawMatches){if(seen.has(match))continue;seen.
add(match);uniqueMatches.push(match)}const viableMatches=viableCandidates(uniqueMatches);if(viableMatches.length>1){const ranking=rankAmbiguousCandidates(viableMatches,
action,selector);const topCandidates=ranking.ranked.slice(0,3).map(entry=>({element_id:getOrCreateElementID(entry.element),tag:entry.element.tagName.toLowerCase(),
text_preview:(entry.element.textContent||"").trim().slice(0,60)||void 0,score:entry.score}));if(ranking.winner){return{element:ranking.winner,match_count:1,match_strategy:"\
ranked_resolution",ranked_candidates:topCandidates}}const sortedCandidates=ranking.ranked.map(entry=>entry.element);return{error:{success:false,action,selector,
error:"ambiguous_target",message:`Selector matches multiple viable elements: ${selector}. Add nth, scope/scope_rect, or use list_interactive element_id/index.`,
match_count:viableMatches.length,match_strategy:"ambiguous_ranked",...scopeRect?{scope_rect_used:scopeRect}:{},candidates:summarizeCandidates(sortedCandidates),
ranked_candidates:topCandidates,suggested_element_id:getOrCreateElementID(ranking.ranked[0].element)}}}const found=viableMatches[0]||null;if(!found)return{error:domError(
"element_not_found",`No element matches selector: ${selector}`)};return{element:found,match_count:1,match_strategy:selectorStrategy(),scope_selector_used:scopeSelectorUsed}}
function resolveActionTarget(){const requestedScope2=(options.scope_selector||"").trim();if(requestedScope2&&!scopeRoot){return{error:domError("scope_not_found",
`No scope element matches selector: ${requestedScope2}`)}}const intentShortcut=resolveIntentShortcut();if(intentShortcut)return intentShortcut;const requestedElementID=(options.
element_id||"").trim();if(requestedElementID){return resolveElementIDTarget(requestedElementID,requestedScope2,scopeSelectorUsed)}const nthResolved=resolveNthTarget();
if(nthResolved)return nthResolved;const ambiguitySensitiveActions=new Set(["click","type","select","check","set_attribute","paste","key_press","focus","scroll_t\
o","hover"]);if(!ambiguitySensitiveActions.has(action)){return resolveUnambiguousTarget()}return resolveRankedTarget()}const resolved=resolveActionTarget();if(resolved.
error)return resolved.error;const el=resolved.element;const resolvedMatchCount=resolved.match_count||1;const resolvedMatchStrategy=resolved.match_strategy||"sel\
ector";const resolvedScopeSelector=resolved.scope_selector_used;const resolvedRankedCandidates=resolved.ranked_candidates;const resolvedAmbiguousMatches=resolved.
ambiguous_matches;function captureViewport(){const w=typeof window!=="undefined"?window:null;const docEl=document?.documentElement;const body=document?.body;return{
scroll_x:Math.round(w?.scrollX??w?.pageXOffset??0),scroll_y:Math.round(w?.scrollY??w?.pageYOffset??0),viewport_width:w?.innerWidth??docEl?.clientWidth??0,viewport_height:w?.
innerHeight??docEl?.clientHeight??0,page_height:Math.max(body?.scrollHeight||0,docEl?.scrollHeight||0)}}function dispatchEventIfPossible(target,event){if(!target)
return;const dispatch=target.dispatchEvent;if(typeof dispatch!=="function")return;dispatch.call(target,event)}function detectOverlayWarning(targetEl){const overlay=findTopmostOverlay();
if(!overlay)return{};if(typeof overlay.contains==="function"&&overlay.contains(targetEl))return{};const overlayInfo=describeOverlay(overlay);return{overlay_warning:`\
An overlay (${overlayInfo.overlay_type}) is covering the page. The action targeted the intended element, but input may be intercepted. Use dismiss_top_overlay t\
o close it first.`,overlay_selector:overlayInfo.overlay_selector}}function mutatingSuccess(node,extra){const overlayInfo=detectOverlayWarning(node);return{success:true,
action,selector,...scopeRect?{scope_rect_used:scopeRect}:{},...extra||{},...overlayInfo.overlay_warning?overlayInfo:{},matched:matchedTarget(node),match_count:resolvedMatchCount,
match_strategy:resolvedMatchStrategy,...resolvedRankedCandidates?{ranked_candidates:resolvedRankedCandidates}:{},viewport:captureViewport()}}function withMutationTracking(fn){
const t0=performance.now();const mutations=[];const observer=new MutationObserver(records=>{mutations.push(...records)});observer.observe(document.body||document.
documentElement,{childList:true,subtree:true,attributes:true,attributeOldValue:!!options.observe_mutations});const result=fn();if(!result.success){observer.disconnect();
return Promise.resolve(result)}function describeMutationSummary(){const added=mutations.reduce((s,m)=>s+m.addedNodes.length,0);const removed=mutations.reduce((s,m)=>s+
m.removedNodes.length,0);const modified=mutations.filter(m=>m.type==="attributes").length;const parts=[];if(added>0)parts.push(`${added} added`);if(removed>0)parts.
push(`${removed} removed`);if(modified>0)parts.push(`${modified} modified`);return{added,removed,modified,summary:parts.length>0?parts.join(", "):"no DOM change\
s"}}function pushNodeEntries(nodes,type,entries,maxEntries){for(let i=0;i<nodes.length&&entries.length<maxEntries;i++){const n=nodes[i];if(n&&n.nodeType===1){const el2=n;
entries.push({type,tag:el2.tagName?.toLowerCase(),id:el2.id||void 0,class:el2.className?.toString()?.slice(0,80)||void 0,text_preview:el2.textContent?.slice(0,100)||
void 0})}}}function collectMutationEntries(maxEntries){const entries=[];for(const m of mutations){if(entries.length>=maxEntries)break;if(m.type==="childList"){pushNodeEntries(
m.addedNodes,"added",entries,maxEntries);pushNodeEntries(m.removedNodes,"removed",entries,maxEntries)}else if(m.type==="attributes"&&m.target.nodeType===1){const el2=m.
target;entries.push({type:"attribute",tag:el2.tagName?.toLowerCase(),id:el2.id||void 0,attribute:m.attributeName||void 0,old_value:m.oldValue?.slice(0,100)||void 0,
new_value:el2.getAttribute(m.attributeName||"")?.slice(0,100)||void 0})}}return entries}return new Promise(resolve=>{let resolved2=false;function finish(){if(resolved2)
return;resolved2=true;observer.disconnect();const totalMs=Math.round(performance.now()-t0);const{added,removed,modified,summary}=describeMutationSummary();const enriched={
...result,dom_summary:summary};if(options.analyze){enriched.timing={total_ms:totalMs};enriched.dom_changes={added,removed,modified,summary};enriched.analysis=`${result.
action} completed in ${totalMs}ms. ${summary}.`}if(options.observe_mutations){enriched.dom_mutations=collectMutationEntries(50)}resolve(enriched)}setTimeout(finish,
80);if(typeof requestAnimationFrame==="function"){requestAnimationFrame(()=>setTimeout(finish,50))}})}function detectRichEditor(node){const el2=node instanceof HTMLElement?
node:node.parentElement||null;if(!el2)return null;const checks=[{selector:".ql-editor",type:"quill"},{selector:".ProseMirror",type:"prosemirror"},{selector:'[da\
ta-contents="true"]',type:"draftjs"},{selector:"[data-editor]",type:"draftjs"},{selector:".mce-content-body",type:"tinymce"},{selector:"#tinymce",type:"tinymce"},
{selector:".ck-editor__editable",type:"ckeditor"}];for(const check of checks){if(typeof el2.matches==="function"&&el2.matches(check.selector)){return{type:check.
type,target:el2}}if(typeof el2.closest==="function"){const ancestor=el2.closest(check.selector);if(ancestor instanceof HTMLElement){return{type:check.type,target:ancestor}}}}
return null}function insertViaRichEditor(_editorType,target,text,clear){const lines=text.split("\n");const htmlParts=[];for(const line of lines){if(line.length>
0){htmlParts.push("<p>"+line.replace(/&/g,"&amp;").replace(/</g,"&lt;").replace(/>/g,"&gt;")+"</p>")}else{htmlParts.push("<p><br></p>")}}const html=htmlParts.join(
"");if(clear){target.innerHTML=html}else{target.insertAdjacentHTML("beforeend",html)}target.dispatchEvent(new Event("input",{bubbles:true}));return{success:true}}
function keyCodeForChar(char){if(char==="\n")return{key:"Enter",code:"Enter",keyCode:13};if(char==="	")return{key:"Tab",code:"Tab",keyCode:9};if(char===" ")return{
key:" ",code:"Space",keyCode:32};const upper=char.toUpperCase();const isLetter=upper>="A"&&upper<="Z";const isDigit=char>="0"&&char<="9";let code;let keyCode;if(isLetter){
code="Key"+upper;keyCode=upper.charCodeAt(0)}else if(isDigit){code="Digit"+char;keyCode=char.charCodeAt(0)}else{code="";keyCode=char.charCodeAt(0)}return{key:char,
code,keyCode}}function dispatchKeySequence(target,char,isContentEditable){const{key,code,keyCode}=keyCodeForChar(char);const shiftKey=char!==char.toLowerCase()&&
char===char.toUpperCase()&&char.toLowerCase()!==char.toUpperCase();const kbOpts={key,code,keyCode,bubbles:true,cancelable:true,shiftKey};target.dispatchEvent(new KeyboardEvent(
"keydown",kbOpts));target.dispatchEvent(new KeyboardEvent("keypress",kbOpts));if(isContentEditable){target.dispatchEvent(new InputEvent("beforeinput",{bubbles:true,
cancelable:true,inputType:"insertText",data:char}));const sel=document.getSelection();if(sel&&sel.rangeCount>0){const range=sel.getRangeAt(0);range.deleteContents();
if(char==="\n"){range.insertNode(document.createElement("br"))}else{range.insertNode(document.createTextNode(char))}range.collapse(false);sel.removeAllRanges();
sel.addRange(range)}target.dispatchEvent(new InputEvent("input",{bubbles:true,inputType:"insertText",data:char}))}target.dispatchEvent(new KeyboardEvent("keyup",
kbOpts))}function insertViaKeyboardSim(node,text){for(const char of text){dispatchKeySequence(node,char,true)}return{success:true}}function isElementOutsideViewport(el2){
if(!(el2 instanceof HTMLElement)||typeof el2.getBoundingClientRect!=="function")return false;const rect=el2.getBoundingClientRect();const{width:viewWidth,height:viewHeight}=viewportSize();
if(viewHeight===0&&viewWidth===0)return false;return rect.bottom<0||rect.top>viewHeight||rect.right<0||rect.left>viewWidth}function autoScrollIfNeeded(el2){if(isElementOutsideViewport(
el2)){el2.scrollIntoView({behavior:"instant",block:"center"});return true}return false}function findInteractiveAncestor(el2){const tag=el2.tagName.toLowerCase();
const role=el2.getAttribute("role")||"";const interactiveTags=new Set(["a","button","input","select","textarea"]);const interactiveRoles=new Set(["button","link",
"menuitem","tab","option","switch"]);if(interactiveTags.has(tag)||interactiveRoles.has(role))return null;if(typeof el2.closest==="function"){const ancestor=el2.
closest('a, button, [role="button"], [role="link"], [role="menuitem"], [role="tab"], input, select, textarea');if(ancestor&&ancestor!==el2)return ancestor}return null}
function detectBlockingOverlay(el2){const dialogs=collectDialogs();if(dialogs.length===0)return null;const topDialog=pickTopDialog(dialogs);if(!topDialog)return null;
if(typeof topDialog.contains==="function"&&topDialog.contains(el2))return null;return topDialog}function describeBlockingOverlay(overlay){const overlayTag=overlay.
tagName.toLowerCase();const overlayRole=overlay.getAttribute("role")||"";const overlayLabel=overlay.getAttribute("aria-label")||"";if(overlayLabel)return`${overlayTag}\
[aria-label="${overlayLabel}"]`;if(overlayRole)return`${overlayTag}[role="${overlayRole}"]`;return overlayTag}function blockedByOverlayError(target){const blockingOverlay=detectBlockingOverlay(
target);if(!blockingOverlay)return null;const overlayDesc=describeBlockingOverlay(blockingOverlay);return domError("blocked_by_overlay",`Element is behind a mod\
al overlay (${overlayDesc}). Use interact({what:"dismiss_top_overlay"}) to close it first.`)}function linkForNewTab(clickTarget){const tag=clickTarget.tagName.toLowerCase();
if(tag==="a")return clickTarget;if(typeof clickTarget.closest==="function"){return clickTarget.closest("a[href]")}return null}function openInNewTab(clickTarget,linkNode,href){
let opened=false;try{if(typeof window!=="undefined"&&typeof window.open==="function"){window.open(href,"_blank","noopener,noreferrer");opened=true}}catch{}if(!opened&&
linkNode instanceof Element){const previousTarget=linkNode.getAttribute("target");linkNode.setAttribute("target","_blank");linkNode.click();if(previousTarget==null){
linkNode.removeAttribute("target")}else{linkNode.setAttribute("target",previousTarget)}}}function structuredTextSections(node){const sections=[];const children=node.
children;for(let i=0;i<children.length&&sections.length<50;i++){const child=children[i];if(!child.tagName)continue;const tag=child.tagName.toLowerCase();const heading=child.
querySelector('h1, h2, h3, h4, h5, h6, [role="heading"], summary, button[aria-expanded]');if(heading){const headerText=heading.innerText?.trim()||"";const ariaExpanded=heading.
getAttribute("aria-expanded");const expanded=ariaExpanded!==null?ariaExpanded==="true":void 0;const contentParts=[];const contentNodes=child.querySelectorAll("p\
, li, span, div, td, pre, code");contentNodes.forEach(cn=>{if(cn!==heading&&!heading.contains(cn)){const t=cn.innerText?.trim();if(t&&t.length>0)contentParts.push(
t)}});sections.push({header:headerText,content:contentParts.join("\n")||(child.innerText?.replace(headerText,"").trim()||""),expanded,tag})}else{const t=child.innerText?.
trim();if(t&&t.length>0){sections.push({content:t,tag})}}}return sections}function buildActionHandlers(node){return{type:()=>withMutationTracking(()=>{const overlayErr=blockedByOverlayError(
node);if(overlayErr)return overlayErr;const text=(options.text||"").replace(/\\n/g,"\n");if(node instanceof HTMLElement&&node.isContentEditable){node.focus();if(options.
clear){const selection=document.getSelection();if(selection){selection.selectAllChildren(node);selection.deleteFromDocument()}}const editor=detectRichEditor(node);
let strategy;if(editor){insertViaRichEditor(editor.type,editor.target,text,!!options.clear);strategy=editor.type+"_native"}else{insertViaKeyboardSim(node,text);
strategy="keyboard_simulation"}return mutatingSuccess(node,{value:node.innerText,insertion_strategy:strategy})}if(!(node instanceof HTMLInputElement)&&!(node instanceof
HTMLTextAreaElement)){return domError("not_typeable",`Element is not an input, textarea, or contenteditable: ${node.tagName}`)}node.focus();for(const char of text){
dispatchKeySequence(node,char,false)}const proto=node instanceof HTMLTextAreaElement?HTMLTextAreaElement:HTMLInputElement;const nativeSetter=Object.getOwnPropertyDescriptor(
proto.prototype,"value")?.set;if(nativeSetter){const newValue=options.clear?text:node.value+text;nativeSetter.call(node,newValue)}else{node.value=options.clear?
text:node.value+text}node.dispatchEvent(new InputEvent("input",{bubbles:true,data:text,inputType:"insertText"}));node.dispatchEvent(new Event("change",{bubbles:true}));
return mutatingSuccess(node,{value:node.value,insertion_strategy:"native_setter"})}),select:()=>withMutationTracking(()=>{const overlayErr=blockedByOverlayError(
node);if(overlayErr)return overlayErr;if(!(node instanceof HTMLSelectElement))return domError("not_select",`Element is not a <select>: ${node.tagName}`);const nativeSelectSetter=Object.
getOwnPropertyDescriptor(HTMLSelectElement.prototype,"value")?.set;if(nativeSelectSetter){nativeSelectSetter.call(node,options.value||"")}else{node.value=options.
value||""}node.dispatchEvent(new Event("change",{bubbles:true}));return mutatingSuccess(node,{value:node.value})}),check:()=>withMutationTracking(()=>{const overlayErr=blockedByOverlayError(
node);if(overlayErr)return overlayErr;if(!(node instanceof HTMLInputElement)||node.type!=="checkbox"&&node.type!=="radio"){return domError("not_checkable",`Elem\
ent is not a checkbox or radio: ${node.tagName} type=${node.type||"N/A"}`)}const desired=options.checked!==void 0?options.checked:true;if(node.checked!==desired){
node.click()}return mutatingSuccess(node,{value:node.checked})}),set_attribute:()=>withMutationTracking(()=>{node.setAttribute(options.name||"",options.value||"");
return mutatingSuccess(node,{value:node.getAttribute(options.name||"")})}),paste:()=>withMutationTracking(()=>{const overlayErr=blockedByOverlayError(node);if(overlayErr)
return overlayErr;if(!(node instanceof HTMLElement))return domError("not_interactive",`Element is not an HTMLElement: ${node.tagName}`);node.focus();if(options.
clear){const selection=document.getSelection();if(selection){selection.selectAllChildren(node);selection.deleteFromDocument()}}const pasteText=(options.text||"").
replace(/\\n/g,"\n");let strategy;const editor=detectRichEditor(node);if(editor&&node.isContentEditable){insertViaRichEditor(editor.type,editor.target,pasteText,
!!options.clear);strategy=editor.type+"_native"}else{const dt=new DataTransfer;dt.setData("text/plain",pasteText);const event=new ClipboardEvent("paste",{clipboardData:dt,
bubbles:true,cancelable:true});node.dispatchEvent(event);strategy="clipboard_event"}return mutatingSuccess(node,{value:node.innerText,insertion_strategy:strategy})}),
key_press:()=>withMutationTracking(()=>{const overlayErr=blockedByOverlayError(node);if(overlayErr)return overlayErr;if(!(node instanceof HTMLElement))return domError(
"not_interactive",`Element is not an HTMLElement: ${node.tagName}`);const key=options.text||options.key||"Enter";if(key==="Tab"||key==="Shift+Tab"){const focusable=Array.
from(node.ownerDocument.querySelectorAll('a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:n\
ot([tabindex="-1"])')).filter(e=>e.offsetParent!==null);const idx=focusable.indexOf(node);const next=key==="Shift+Tab"?focusable[idx-1]:focusable[idx+1];if(next){
next.focus();return mutatingSuccess(node,{value:key})}return mutatingSuccess(node,{value:key,message:"No next focusable element"})}const keyMap={Enter:{key:"Ent\
er",code:"Enter",keyCode:13},Tab:{key:"Tab",code:"Tab",keyCode:9},Escape:{key:"Escape",code:"Escape",keyCode:27},Backspace:{key:"Backspace",code:"Backspace",keyCode:8},
ArrowDown:{key:"ArrowDown",code:"ArrowDown",keyCode:40},ArrowUp:{key:"ArrowUp",code:"ArrowUp",keyCode:38},Space:{key:" ",code:"Space",keyCode:32}};const mapped=keyMap[key]||
{key,code:key,keyCode:0};node.dispatchEvent(new KeyboardEvent("keydown",{key:mapped.key,code:mapped.code,keyCode:mapped.keyCode,bubbles:true}));node.dispatchEvent(
new KeyboardEvent("keypress",{key:mapped.key,code:mapped.code,keyCode:mapped.keyCode,bubbles:true}));node.dispatchEvent(new KeyboardEvent("keyup",{key:mapped.key,
code:mapped.code,keyCode:mapped.keyCode,bubbles:true}));return mutatingSuccess(node,{value:key})})}}const handlers=buildActionHandlers(el);const handler=handlers[action];
if(!handler){return domError("unknown_action",`Unknown DOM action: ${action}`)}const rawResult=handler();if(!resolvedAmbiguousMatches)return rawResult;if(rawResult instanceof
Promise){return rawResult.then(r=>{if(r&&typeof r==="object"&&r.success){return{...r,ambiguous_matches:resolvedAmbiguousMatches}}return r})}if(rawResult&&typeof rawResult===
"object"&&rawResult.success){return{...rawResult,ambiguous_matches:resolvedAmbiguousMatches}}return rawResult}
// jscpd:ignore-end
