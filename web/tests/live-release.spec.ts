import { test, expect } from '@playwright/test';
import { LOAD_PATHS_SOURCE } from '../lib/solver-prompt';
test('published Load Paths and preserved Quiet Echoes render from the live API',async ({page,request})=>{
 test.skip(process.env.LIVE_RELEASE_CHECK!=='1','Explicit read-only production smoke check');
 const load=await request.get('/v1/challenges/load-paths');expect(load.ok()).toBe(true);
 const record=await load.json();expect(record.status).toBe('published');expect(record.sourceCommit).toBe(LOAD_PATHS_SOURCE);
 expect(record.submissions.filter((s:{verificationStatus:string})=>s.verificationStatus==='platform_verified').length).toBeGreaterThanOrEqual(3);
 await page.goto('/challenges/load-paths');
 await expect(page.getByRole('heading',{name:'Load Paths',exact:true})).toBeVisible();
 await expect(page.getByRole('region',{name:'Load Paths reference explorer'})).toBeVisible();
 await expect(page.getByRole('button',{name:'Participate',exact:true})).toBeVisible();
 await page.screenshot({path:'/tmp/science-ladder-load-paths-live.png',fullPage:true});
 const quiet=await request.get('/v1/challenges/quiet-echoes-labs512');expect(quiet.ok()).toBe(true);
 const old=await quiet.json();expect(old.sourceCommit).toBe('f42f527e97563b1c068a1835732c6da44f21223f');
 expect(old.lockDigest).toBe('sha256:ae2103aca32a90c6bb166745cbd6aa2fcfbc3381fe01a1a27f2190afb7bfbbd4');
 expect(old.submissions.length).toBeGreaterThanOrEqual(3);
});
