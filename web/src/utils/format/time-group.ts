import i18n, { appIntlLocale } from '@/i18n';

const { t } = i18n.global;

export function groupByTime<T extends { update_time: number }>(
  items: T[],
  now: number = Date.now()
): Map<string, T[]> {
  const groups = new Map<string, T[]>();
  const oneDay = 86400000;

  for (const item of items) {
    const diff = now - item.update_time * 1000;
    let group: string;
    // 这里的中文**既是分组键又是显示文字**。改成 t() 是安全的：全仓只有 RecentDocs 消费它，
    // 而它只把键当标题渲染、从不按字面量比较（grep '今天' 只命中这一处）。
    if (diff < oneDay) group = t('time.today');
    else if (diff < 7 * oneDay) group = t('time.last7Days');
    else group = t('time.earlier');

    if (!groups.has(group)) groups.set(group, []);
    groups.get(group)!.push(item);
  }
  return groups;
}

const GREETING_POOL = {
  night: [
    '夜深了，星光不负赶路人',
    '夜色温柔，愿你有个好梦',
    '月亮不睡我不睡，但你要早睡',
    '深夜的宁静，是思考的最佳时刻',
    '星河滚烫，你是人间理想',
    '城市睡了，你还醒着',
    '深夜的灯，照亮前行的路',
    '愿长夜无梦，愿你安眠',
    '凌晨的清醒，是灵魂的独处',
    '夜已深，心事也该放下了',
    '万籁俱寂，唯有心跳与星光共鸣',
    '熬夜伤身，但灵感常在深夜造访',
    '夜幕低垂，愿你卸下所有疲惫',
    '深夜的你，值得被温柔以待',
    '星月作伴，好梦可期',
    '夜色如墨，心却澄明',
    '愿你的夜晚，有灯有故事',
    '夜深人静，正是与自己对话的时候',
    '晚风轻拂，愿你安然入眠',
    '深夜的思绪，是明天的伏笔',
  ],
  morning: [
    '早上好，新的一天充满希望',
    '早安，愿你今天元气满满',
    '晨光熹微，又是美好的一天',
    '每一个清晨，都是生活的重启键',
    '早安，今天也要闪闪发光',
    '新的一天，从一杯咖啡开始',
    '日出有盼，日落有念',
    '早安，世界因你而明亮',
    '清晨的第一缕阳光，送给你',
    '愿你被今日温柔以待',
    '早起的人，配得上所有的美好',
    '今天也是值得努力的一天',
    '早安，愿你心中有光，脚下有路',
    '一日之计在于晨，愿你把握当下',
    '新的一天，新的可能',
    '晨光里，所有的美好都在苏醒',
    '早安，愿你今天比昨天更快乐',
    '清风拂面，好运自来',
    '愿你晨起有微笑，梦里有星辰',
    '今日限定：无限可能',
  ],
  noon: [
    '中午好，记得按时吃饭',
    '午安，适当休息才能走得更远',
    '午间充电中，下午继续加油',
    '午餐时间，别忘了犒劳自己',
    '忙了一上午，该歇歇了',
    '午后小憩，养精蓄锐',
    '吃饱了才有力气追梦',
    '正午阳光正好，心情也要明媚',
    '午休片刻，效率翻倍',
    '午间时光，给自己放个假',
    '午饭要吃饱，心情要放好',
    'midday 是属于自己的小确幸',
    '中午好，愿你午后不犯困',
    '午间一碗热饭，治愈所有',
    '阳光正好，微风不燥',
    '午休一下，再出发',
    '午后时光，享受片刻宁静',
    '午安，愿被这温柔世界善待',
  ],
  afternoon: [
    '下午好，保持专注',
    '午后时光，效率正当时',
    '下午茶时间，给自己充个电',
    '阳光正好，心情也要明媚',
    '下午是创作的最佳时刻',
    '不急不躁，稳扎稳打',
    '下午茶一杯，烦恼全飞',
    '午后微醺的不是茶，是生活',
    '下午的阳光，适合思考和沉淀',
    '愿你下午的工作顺顺利利',
    '下午好，愿你保持热爱',
    '阳光斜照，正是奋斗好时光',
    '下午的效率，决定一天的高度',
    '愿你午后不犯困，事事都顺心',
    '下午好，坚持就是胜利',
    '午后清醒，灵感迸发',
    '阳光透过窗，时光正温柔',
    '愿你的下午，有光也有梦',
    '午后时光，不负自己',
  ],
  evening: [
    '晚上好，今天过得怎么样',
    '夜幕降临，该放松一下了',
    '愿你被世界温柔以待',
    '一天辛苦啦，好好休息',
    '夜晚是属于自己的时光',
    '愿你今晚有甜甜的梦',
    '生活明朗，万物可爱',
    'Evening，卸下疲惫，拥抱自己',
    '夜色温柔，适合复盘与展望',
    '今天的故事写完了，明天继续',
    '晚风习习，愿你心情美丽',
    '夜晚的灯光，是归人的温暖',
    '愿你今晚无琐事打扰',
    '日落归山海，你归家',
    '今晚的月亮，分你一半',
    '愿你在夜色中找到平静',
    '晚上好，愿今天的你比昨天更好',
    '星光不问赶路人，时光不负有心人',
    '夜幕温柔，好梦将至',
    '愿你的夜晚，有故事有温度',
  ],
} as const;

const QUOTE_POOL = {
  night: [
    '「愿你所有的努力，都能在黑夜里化作星光」',
    '「独处是与灵魂对话的最好时刻」',
    '「今天的疲惫，都是明天的铺垫」',
    '「黑夜给了我黑色的眼睛，我却用它寻找光明」',
    '「寂静的夜，是灵感最好的温床」',
    '「世界很大，愿你好梦」',
    '「睡吧，明天又是充满可能的一天」',
  ],
  morning: [
    '「晨光熹微，万物可期」',
    '「每一个不曾起舞的日子，都是对生命的辜负」',
    '「今天的努力，是明天最好的礼物」',
    '「愿你眼中有星辰，心中有大海」',
    '「生活从不亏待每一个努力的人」',
    '「向阳而生，不负韶华」',
    '「人间值得，未来可期」',
  ],
  noon: [
    '「好好吃饭，是最简单的幸福」',
    '「生活，就是一边受苦一边热爱」',
    '「休息是为了走更远的路」',
    '「人间烟火气，最抚凡人心」',
    '「平凡的日子里，藏着最真的幸福」',
    '「愿你被这世界温柔以待」',
    '「知足常乐，心宽体胖」',
  ],
  afternoon: [
    '「最好的时光，是现在」',
    '「种一棵树最好的时间是十年前，其次是现在」',
    '「不要等待机会，而要创造机会」',
    '「保持热爱，奔赴山海」',
    '「心之所向，素履以往」',
    '「愿你成为自己的太阳」',
    '「努力的意义，是为了遇见更好的自己」',
  ],
  evening: [
    '「愿你一生温暖纯良，不舍爱与自由」',
    '「星光不问赶路人，时光不负有心人」',
    '「把一个人的名字写在纸上，不如写在心上」',
    '「愿你所有快乐，无需假装」',
    '「归去来兮，田园将芜胡不归」',
    '「慢一点，世界会更清晰」',
    '「夜晚是最诚实的朋友」',
  ],
} as const;

function pickRandom<T>(arr: readonly T[]): T {
  return arr[Math.floor(Math.random() * arr.length)] ?? arr[0]!;
}

// 英文档另起一套，不是把上面逐句翻过来。
// 这类一句话是**语气**而不是术语：直译的英文（"星河滚烫，你是人间理想" → "The galaxy is hot..."）
// 读起来像机翻，比不显示更糟。所以按同样的时段结构写一套英文原句，条数可以少、语气必须对。
const GREETING_POOL_EN = {
  night: [
    'Late night — the good ideas usually show up now',
    'Still up? Be gentle with yourself',
    'The city is quiet. Your turn to rest',
    'One more page, then sleep',
    'Night shift on something that matters',
    'Wrap up. Tomorrow needs you sharp',
    'The best thinking happens after midnight',
    'Lights low, mind clear',
  ],
  morning: [
    'Good morning — fresh page, no writing on it yet',
    'Morning. Start with the one thing',
    'A good coffee and a clear list',
    'Rise and document',
    'Today is the day you said yesterday',
    'Slow start, steady finish',
    'Morning: the only uninterrupted hours you get',
    'New day, new notes',
  ],
  noon: [
    'Good afternoon — eat first, optimize later',
    'Midday check-in: what is actually left today?',
    'Lunch break. The work will wait',
    'Step away from the screen for ten minutes',
    'Half the day gone. Protect the second half',
    'Rest now so the afternoon has something to give',
    'Do the hard thing before the post-lunch slump',
    'Slow down, then speed up',
  ],
  afternoon: [
    'Good afternoon — keep one thread open at a time',
    'Deep work window: guard it',
    'Afternoon is for finishing, not starting',
    'Small step, logged step',
    'Write it down before you forget it',
    'One more review and this is done',
    'Momentum beats intensity',
    'Finish something today, however small',
  ],
  evening: [
    'Good evening — how did today go?',
    'Close the tabs. The notes are safe',
    'Evening is for reading, not shipping',
    'Log what you learned and log off',
    'Tomorrow starts with a clean list',
    'Wind down — the hard part is over',
    'Review the day in three lines',
    'Quiet hours. Let the ideas settle',
  ],
} as const;

const QUOTE_POOL_EN = {
  night: [
    '"A written note is a promise you keep for tomorrow."',
    '"Ideas are cheap. Filed ideas are not."',
    '"Rest is part of the work, not a reward for it."',
    '"The archive you build tonight answers questions you have not asked yet."',
    '"Sleep on it — but write it down first."',
  ],
  morning: [
    '"You do not rise to your tools; you fall to your habits."',
    '"A knowledge base is a letter to your future self."',
    '"Start with the question, not the document."',
    '"Clarity in the morning saves meetings in the afternoon."',
    '"The best time to fix a document was when you last read it."',
  ],
  noon: [
    '"Take the break. Fatigue is a bug, not a badge."',
    '"Half of good work is knowing what not to reopen today."',
    '"Eat, then decide."',
    '"A short walk finds what an hour of staring cannot."',
    '"Protect the afternoon the way you protect a meeting."',
  ],
  afternoon: [
    '"Small, finished beats large, planned."',
    '"Write it once, reuse it all quarter."',
    '"If it is worth remembering, it is worth searching later."',
    '"Depth now, breadth next week."',
    '"The second draft is where the thinking starts."',
  ],
  evening: [
    '"End the day with one sentence about it."',
    '"What you capture tonight, you will not rebuild on Friday."',
    '"A tidy note is a kindness to whoever reads it next — usually you."',
    '"Done is a feature."',
    '"The inbox is not the archive."',
  ],
} as const;

export interface GreetingResult {
  greeting: string;
  quote: string;
  period: 'night' | 'morning' | 'noon' | 'afternoon' | 'evening';
}

export function getTimeGreeting(): GreetingResult {
  const hour = new Date().getHours();
  let period: GreetingResult['period'];
  if (hour < 6) period = 'night';
  else if (hour < 12) period = 'morning';
  else if (hour < 14) period = 'noon';
  else if (hour < 18) period = 'afternoon';
  else period = 'evening';

  // 问候语与题句是**运行时**挑的，所以它跟着界面开关走，而不是 navigator.language。
  const zh = appIntlLocale().toLowerCase().startsWith('zh');
  return {
    greeting: pickRandom((zh ? GREETING_POOL : GREETING_POOL_EN)[period]),
    quote: pickRandom((zh ? QUOTE_POOL : QUOTE_POOL_EN)[period]),
    period,
  };
}