package text2sql

import (
	"regexp"
	"strings"
	"unicode"
)

type EntityRecognizer struct {
	kb *KnowledgeBase
}

func NewEntityRecognizer(kb *KnowledgeBase) *EntityRecognizer {
	return &EntityRecognizer{kb: kb}
}

type RecognizedInput struct {
	Tables     []string            `json:"tables"`
	Fields     map[string][]string `json:"fields"`
	Conditions []Condition         `json:"conditions"`
	Intent     string              `json:"intent"`
	RawText    string              `json:"raw_text"`
}

type Condition struct {
	Table string `json:"table"`
	Field string `json:"field"`
	Op    string `json:"op"`
	Value string `json:"value"`
}

// 常见业务关键词到英文的映射
var bizKeywordMap = map[string]string{
	"用户": "user", "客户": "customer", "订单": "order",
	"产品": "product", "商品": "product", "库存": "stock",
	"仓库": "warehouse", "供应商": "supplier", "员工": "employee",
	"部门": "department", "角色": "role", "权限": "permission",
	"日志": "log", "配置": "config", "设置": "setting",
	"分类": "category", "标签": "tag", "文章": "article",
	"文档": "document", "评论": "comment", "消息": "message",
	"通知": "notification", "任务": "task", "流程": "process",
	"审批": "approval", "报表": "report", "统计": "stat",
	"账单": "bill", "支付": "payment", "交易": "transaction",
	"账户": "account", "地址": "address", "手机": "phone",
	"电话": "phone", "邮箱": "email", "邮件": "email",
	"名称": "name", "标题": "title", "状态": "status",
	"类型": "type", "时间": "time", "日期": "date",
	"创建": "create", "更新": "update", "删除": "delete",
	"编号": "no", "编码": "code", "数量": "quantity",
	"金额": "amount", "价格": "price", "费用": "fee",
	"积分": "point", "优惠券": "coupon", "活动": "activity",
	"促销": "promotion", "渠道": "channel", "来源": "source",
	"地区": "region", "城市": "city", "省份": "province",
	"国家": "country", "集团": "group", "公司": "company",
	"企业": "enterprise", "项目": "project", "合同": "contract",
	"协议": "agreement", "发票": "invoice", "税务": "tax",
	"财务": "finance", "会计": "accounting", "银行": "bank",
	"卡号": "card", "会员": "member", "等级": "level",
	"评分": "score", "评价": "review", "反馈": "feedback",
	"投诉": "complaint", "售后": "aftersale", "退款": "refund",
	"退货": "return", "物流": "logistics", "快递": "express",
	"运费": "freight", "配送": "delivery", "收货": "receive",
	"发货": "ship", "入库": "inbound", "出库": "outbound",
	"盘点": "inventory", "调拨": "transfer", "采购": "purchase",
	"销售": "sale", "营销": "marketing", "推广": "promotion",
	"广告": "ad", "访问": "visit", "浏览": "view",
	"点击": "click", "收藏": "favorite", "关注": "follow",
	"分享": "share", "推荐": "recommend", "搜索": "search",
	"关键词": "keyword", "品牌": "brand", "规格": "spec",
	"型号": "model", "单位": "unit", "计量": "measure",
	"重量": "weight", "体积": "volume", "尺寸": "size",
	"颜色": "color", "图片": "image", "文件": "file",
	"附件": "attachment", "备注": "remark", "描述": "description",
	"详情": "detail", "摘要": "summary", "内容": "content",
	"模板": "template", "规则": "rule", "策略": "strategy",
	"计划": "plan", "目标": "target", "指标": "indicator",
	"业绩": "performance", "提成": "commission", "薪资": "salary",
	"工资": "wage", "考勤": "attendance", "请假": "leave",
	"加班": "overtime", "招聘": "recruit", "面试": "interview",
	"入职": "onboard", "离职": "resign", "培训": "training",
	"考核": "assessment", "绩效": "kpi", "资产": "asset",
	"设备": "device", "物料": "material", "原材料": "raw",
	"成品": "finished", "半成品": "semi", "工序": "process",
	"工艺": "craft", "质量": "quality", "检验": "inspect",
	"检测": "detect", "合格": "qualified", "不合格": "unqualified",
	"缺陷": "defect", "故障": "fault", "维修": "repair",
	"保养": "maintain", "工单": "workorder", "生产": "produce",
	"制造": "manufacture", "排产": "schedule", "产能": "capacity",
	"效率": "efficiency", "损耗": "loss", "报废": "scrap",
	"成本": "cost", "预算": "budget", "核算": "accounting",
	"利润": "profit", "收入": "revenue", "支出": "expense",
	"负债": "liability", "现金": "cash", "存款": "deposit",
	"贷款": "loan", "利息": "interest", "汇率": "rate",
	"币种": "currency", "科目": "subject", "凭证": "voucher",
	"账簿": "ledger", "审计": "audit", "风险": "risk",
	"合规": "compliance", "法律": "legal", "诉讼": "lawsuit",
	"仲裁": "arbitration", "保险": "insurance", "索赔": "claim",
	"保单": "policy", "投保人": "insured", "被保人": "beneficiary",
	"受益人": "beneficiary", "险种": "insurance_type", "保费": "premium",
	"保额": "coverage", "期限": "term", "生效": "effective",
	"失效": "expired", "续保": "renewal", "退保": "surrender",
	"理赔": "claim", "核保": "underwriting", "保全": "conservation",
	"回访": "visit", "咨询": "consult", "建议": "suggestion",
	"满意度": "satisfaction", "服务": "service", "响应": "response",
	"处理": "handle", "解决": "resolve", "关闭": "close",
	"重启": "reopen", "升级": "escalate", "转派": "transfer",
	"协同": "collaborate", "知识库": "knowledge", "问题": "issue",
	"方案": "solution", "案例": "case", "经验": "experience",
	"总结": "summary", "汇报": "report", "会议": "meeting",
	"纪要": "minutes", "决议": "resolution", "行动": "action",
	"跟踪": "track", "督办": "supervise", "晋升": "promote",
	"调岗": "transfer", "轮岗": "rotate", "继任": "succession",
	"人才": "talent", "梯队": "pipeline", "储备": "reserve",
	"培养": "cultivate", "发展": "develop", "规划": "plan",
	"职业": "career", "导师": "mentor", "教练": "coach",
	"辅导": "coach", "学习": "learn", "课程": "course",
	"考试": "exam", "认证": "certification", "资质": "qualification",
	"技能": "skill", "能力": "competency", "素质": "quality",
	"画像": "portrait", "图谱": "graph", "关系": "relation",
	"网络": "network", "社交": "social", "圈子": "circle",
	"社群": "community", "论坛": "forum", "话题": "topic",
	"帖子": "post", "博客": "blog", "专栏": "column",
	"频道": "channel", "直播间": "live", "短视频": "short_video",
	"音频": "audio", "播客": "podcast", "音乐": "music",
	"歌曲": "song", "专辑": "album", "歌手": "artist",
	"乐队": "band", "导演": "director", "演员": "actor",
	"编剧": "screenwriter", "翻译": "translate", "本地化": "localize",
	"时区": "timezone", "日历": "calendar", "节假日": "holiday",
	"工作日": "workday", "班次": "shift", "打卡": "punch",
	"签到": "checkin", "签退": "checkout", "外勤": "field",
	"出差": "business_trip", "调休": "compensatory", "年假": "annual_leave",
	"病假": "sick_leave", "事假": "personal_leave", "婚假": "marriage_leave",
	"产假": "maternity_leave", "陪产假": "paternity_leave", "丧假": "bereavement_leave",
	"工伤假": "injury_leave", "哺乳假": "nursing_leave", "探亲假": "family_visit_leave",
	"公休": "public_holiday", "法定": "statutory", "福利假": "welfare_leave",
	"带薪": "paid", "无薪": "unpaid", "降职": "demotion",
	"辞退": "dismissal", "开除": "termination", "裁员": "layoff",
	"退休": "retirement", "返聘": "rehire", "内退": "early_retirement",
	"病退": "medical_retirement", "工伤": "work_injury", "职业病": "occupational_disease",
	"体检": "physical_exam", "健康": "health", "社保": "social_insurance",
	"公积金": "housing_fund", "医保": "medical_insurance", "养老": "pension",
	"失业": "unemployment", "工伤险": "injury_insurance", "生育险": "maternity_insurance",
	"补充医疗": "supplementary_medical", "商业保险": "commercial_insurance", "意外险": "accident_insurance",
	"重疾险": "critical_illness", "寿险": "life_insurance", "年金": "annuity",
	"分红": "dividend", "万能": "universal", "投连": "investment_linked",
	"信托": "trust", "基金": "fund", "股票": "stock",
	"债券": "bond", "期货": "futures", "期权": "options",
	"外汇": "forex", "贵金属": "precious_metal", "大宗商品": "commodity",
	"衍生品": "derivative", "结构性存款": "structured_deposit", "理财": "wealth_management",
	"资管": "asset_management", "投行": "investment_bank", "券商": "securities",
	"公募": "public_fund", "私募": "private_fund", "上市": "list",
	"退市": "delist", "并购": "ma", "收购": "acquisition",
	"合并": "merge", "分拆": "spinoff", "重组": "restructure",
	"破产": "bankruptcy", "清算": "liquidation", "解散": "dissolve",
	"吊销": "revoke", "异常": "abnormal", "失信": "dishonest",
	"被执行": "executed", "限制消费": "consumption_limit", "签证": "visa",
	"护照": "passport", "居留": "residence", "永居": "permanent_residence",
	"国籍": "nationality", "户籍": "household", "籍贯": "native_place",
	"出生地": "birthplace", "民族": "ethnicity", "政治面貌": "political_status",
	"学历": "education", "学位": "degree", "学校": "school",
	"专业": "major", "毕业": "graduate", "在读": "studying",
	"肄业": "dropout", "结业": "complete", "函授": "correspondence",
	"自考": "self_study", "网络教育": "online_education", "成人教育": "adult_education",
	"继续教育": "continuing_education", "职业培训": "vocational_training", "技能鉴定": "skill_assessment",
	"职业资格": "vocational_qualification", "执业资格": "practicing_qualification", "职称": "professional_title",
	"职务": "position", "岗位": "post", "职级": "grade",
	"薪级": "salary_grade", "档级": "band", "系数": "coefficient",
	"基薪": "base_salary", "奖金": "bonus", "津贴": "allowance",
	"补贴": "subsidy", "加班费": "overtime_pay", "年终奖": "year_end_bonus",
	"季度奖": "quarterly_bonus", "项目奖": "project_bonus", "股权": "equity",
	"激励": "incentive", "福利": "benefit", "餐补": "meal_allowance",
	"交通补": "transport_allowance", "通讯补": "communication_allowance", "住房补": "housing_allowance",
	"高温补": "high_temp_allowance", "取暖补": "heating_allowance", "节日补": "holiday_allowance",
	"生日补": "birthday_allowance", "结婚补": "marriage_allowance", "生育补": "birth_allowance",
	"丧葬补": "funeral_allowance", "旅游": "travel", "团建": "team_building",
	"书籍": "books", "软件": "software", "硬件": "hardware",
	"家具": "furniture", "办公用品": "office_supplies", "耗材": "consumables",
	"礼品": "gift", "招待": "entertainment", "差旅": "travel_expense",
	"住宿": "accommodation", "机票": "flight", "火车票": "train",
	"出租车": "taxi", "网约车": "ride_hailing", "公交": "bus",
	"地铁": "subway", "停车": "parking", "加油": "fuel",
	"过路费": "toll", "年检": "annual_inspection", "违章": "violation",
	"事故": "accident", "救援": "rescue", "代驾": "designated_driver",
	"租车": "car_rental", "用车": "car_usage", "公务车": "official_car",
	"班车": "shuttle", "通勤": "commute", "食堂": "canteen",
	"餐饮": "catering", "外卖": "takeout", "超市": "supermarket",
	"便利店": "convenience_store", "商场": "mall", "网购": "online_shopping",
	"电商": "ecommerce", "零售": "retail", "批发": "wholesale",
	"分销": "distribution", "代理": "agency", "经销": "dealer",
	"加盟": "franchise", "直营": "direct", "联营": "joint",
	"合伙": "partnership", "合资": "joint_venture", "独资": "sole",
	"国有": "state_owned", "民营": "private", "外资": "foreign",
	"总公司": "headquarters", "分公司": "branch", "子公司": "subsidiary",
	"办事处": "office", "门店": "store", "网点": "outlet",
	"专柜": "counter", "展厅": "showroom", "体验馆": "experience_center",
	"服务中心": "service_center", "维修站": "repair_station", "加油站": "gas_station",
	"充电站": "charging_station", "换电站": "battery_swap", "停车场": "parking_lot",
	"收费站": "toll_station", "检查站": "checkpoint", "安检": "security_check",
	"海关": "customs", "边检": "border", "检疫": "quarantine",
	"校准": "calibration", "认可": "accreditation", "注册": "registration",
	"备案": "filing", "许可": "license", "牌照": "plate",
	"证照": "certificate", "批文": "approval", "批复": "reply",
	"函": "letter", "通报": "circular", "公告": "announcement",
	"公示": "publicity", "声明": "statement", "承诺": "commitment",
	"保证": "guarantee", "担保": "guarantee", "抵押": "mortgage",
	"质押": "pledge", "留置": "lien", "定金": "deposit",
	"违约金": "penalty", "赔偿金": "compensation", "损失": "loss",
	"损害": "damage", "侵权": "infringement", "违约": "breach",
	"违法": "violation", "犯罪": "crime", "违规": "noncompliance",
	"违纪": "discipline", "处分": "punishment", "处罚": "penalty",
	"罚款": "fine", "滞纳金": "late_fee", "罚息": "penalty_interest",
	"复利": "compound_interest", "本金": "principal", "本息": "principal_interest",
	"余额": "balance", "欠款": "arrears", "透支": "overdraft",
	"逾期": "overdue", "不良": "nonperforming", "坏账": "bad_debt",
	"呆账": "dormant_debt", "核销": "writeoff", "转让": "transfer",
	"利率": "interest_rate", "基准利率": "benchmark_rate", "降准": "rrr_cut",
	"加息": "rate_hike", "降息": "rate_cut", "量化宽松": "qe",
	"紧缩": "tightening", "宽松": "easing", "中性": "neutral",
	"稳健": "prudent", "积极": "proactive", "消极": "passive",
	"扩张": "expansion", "收缩": "contraction", "通胀": "inflation",
	"通缩": "deflation", "滞胀": "stagflation", "碳排放": "carbon_emission",
	"碳中和": "carbon_neutral", "碳达峰": "carbon_peak", "绿色金融": "green_finance",
	"可持续": "sustainable", "社会责任": "csr", "公益": "charity",
	"捐赠": "donation", "志愿": "volunteer", "扶贫": "poverty",
	"乡村振兴": "rural", "共同富裕": "common_prosperity", "双循环": "dual_circulation",
	"一带一路": "belt_road", "自贸区": "free_trade_zone", "自贸港": "free_trade_port",
	"关税": "tariff", "壁垒": "barrier", "倾销": "dumping",
	"反倾销": "anti_dumping", "反补贴": "countervailing", "保障措施": "safeguard",
	"争端解决": "dispute", "调解": "mediation", "上诉": "appeal",
	"再审": "retrial", "执行": "execution", "先予执行": "advance_execution",
	"强制执行": "forced_execution", "拘留": "detention", "逮捕": "arrest",
	"取保候审": "bail", "监视居住": "residential_surveillance", "缓刑": "probation",
	"假释": "parole", "减刑": "sentence_reduction", "特赦": "amnesty",
	"大赦": "pardon", "赦免": "remission", "豁免": "exemption",
	"免责": "disclaimer", "不可抗力": "force_majeure", "情势变更": "change_circumstances",
	"重大误解": "major_misunderstanding", "显失公平": "gross_unfairness", "欺诈": "fraud",
	"胁迫": "coercion", "乘人之危": "taking_advantage", "恶意串通": "malicious_collusion",
	"损害国家利益": "national_interest", "公共利益": "public_interest", "社会秩序": "social_order",
	"公序良俗": "public_order", "道德": "morality", "伦理": "ethics",
	"诚信": "integrity", "信用": "credit", "信誉": "reputation",
	"商誉": "goodwill", "商标": "trademark", "专利": "patent",
	"著作权": "copyright", "版权": "copyright", "商业秘密": "trade_secret",
	"域名": "domain", "肖像权": "portrait_right", "名誉权": "reputation_right",
	"隐私权": "privacy_right", "个人信息": "personal_info", "敏感信息": "sensitive_info",
	"重要数据": "important_data", "核心数据": "core_data", "国家秘密": "state_secret",
	"工作秘密": "work_secret", "内部资料": "internal", "公开": "public",
	"保密": "confidential", "机密": "classified", "绝密": "top_secret",
	"密级": "classification", "保密期限": "confidentiality_period", "知悉范围": "access_scope",
	"定密": "classification_decision", "解密": "declassification", "变更密级": "classification_change",
	"泄露": "leak", "窃取": "theft", "刺探": "espionage",
	"收买": "bribery", "非法提供": "illegal_provision", "非法获取": "illegal_acquisition",
	"非法持有": "illegal_possession", "非法复制": "illegal_copy", "非法记录": "illegal_recording",
	"非法存储": "illegal_storage", "非法处理": "illegal_processing", "非法传输": "illegal_transmission",
	"非法买卖": "illegal_trading", "非法披露": "illegal_disclosure", "间谍": "spy",
	"叛国": "treason", "分裂": "secession", "颠覆": "subversion",
	"恐怖": "terrorism", "极端": "extremism", "腐败": "corruption",
	"贪污": "embezzlement", "受贿": "bribery", "行贿": "bribe_giving",
	"挪用": "misappropriation", "侵占": "occupation", "洗钱": "money_laundering",
	"走私": "smuggling", "偷税": "tax_evasion", "漏税": "tax_avoidance",
	"骗税": "tax_fraud", "抗税": "tax_resistance", "欠税": "tax_arrears",
	"逃税": "tax_evade", "避税": "tax_avoid", "税收筹划": "tax_planning",
	"税务稽查": "tax_audit", "税务检查": "tax_inspection", "税务处罚": "tax_penalty",
	"税务复议": "tax_reconsideration", "税务诉讼": "tax_litigation", "税务赔偿": "tax_compensation",
	"税务代理": "tax_agency", "税务顾问": "tax_advisor", "税务师": "tax_consultant",
	"会计师": "accountant", "审计师": "auditor", "律师": "lawyer",
	"评估师": "appraiser", "咨询师": "consultant", "分析师": "analyst",
	"架构师": "architect", "工程师": "engineer", "程序员": "programmer",
	"开发": "developer", "测试": "tester", "运维": "ops",
	"产品经理": "pm", "项目经理": "project_manager", "运营": "operation",
	"市场": "marketing", "商务": "business", "供应链": "supply_chain",
	"仓储": "warehouse", "客服": "customer_service", "技术支持": "tech_support",
	"实施": "implementation", "交付": "delivery", "验收": "acceptance",
	"维保": "maintenance", "质保": "warranty", "保内": "in_warranty",
	"保外": "out_warranty", "延保": "extended_warranty", "换新": "replacement",
	"退换": "return_exchange", "补偿": "compensation", "赔偿": "indemnity",
	"订金": "deposit", "预付款": "advance_payment", "进度款": "progress_payment",
	"尾款": "final_payment", "质保金": "retention_money", "保证金": "security_deposit",
	"押金": "deposit", "租金": "rent", "物业费": "property_fee",
	"水电费": "utilities", "燃气费": "gas_fee", "网费": "internet_fee",
	"电话费": "phone_fee", "停车费": "parking_fee", "维修费": "maintenance_fee",
	"保洁费": "cleaning_fee", "安保费": "security_fee", "绿化费": "greening_fee",
	"垃圾费": "garbage_fee", "排污费": "sewage_fee", "检测费": "testing_fee",
	"检验费": "inspection_fee", "鉴定费": "appraisal_fee", "公证费": "notary_fee",
	"仲裁费": "arbitration_fee", "诉讼费": "litigation_fee", "保全费": "preservation_fee",
	"执行费": "execution_fee", "评估费": "evaluation_fee", "拍卖费": "auction_fee",
	"变卖费": "sale_fee", "保管费": "storage_fee", "运输费": "transport_fee",
	"装卸费": "loading_fee", "包装费": "packing_fee", "保险费": "insurance_fee",
	"增值税": "vat", "消费税": "consumption_tax", "营业税": "business_tax",
	"所得税": "income_tax", "企业所得税": "corporate_tax", "个人所得税": "personal_tax",
	"印花税": "stamp_tax", "房产税": "property_tax", "土地使用税": "land_tax",
	"契税": "deed_tax", "车船税": "vehicle_tax", "资源税": "resource_tax",
	"环境税": "environment_tax", "耕地占用税": "farmland_tax", "城建税": "city_tax",
	"教育费附加": "education_surcharge", "地方教育附加": "local_education", "水利基金": "water_fund",
	"价格调节基金": "price_fund", "文化事业费": "culture_fee", "残疾人就业保障金": "disability_fund",
	"工会经费": "union_fund", "社保费": "social_security", "补充养老": "supplementary_pension",
	"团体险": "group_insurance", "雇主责任险": "employer_liability", "公众责任险": "public_liability",
	"产品责任险": "product_liability", "职业责任险": "professional_liability", "董责险": "d_and_o",
	"信用保险": "credit_insurance", "保证保险": "guarantee_insurance", "货物运输险": "cargo_insurance",
	"船舶险": "hull_insurance", "航空险": "aviation_insurance", "家财险": "home_insurance",
	"健康险": "health_insurance", "医疗险": "medical_insurance", "年金险": "annuity_insurance",
	"分红险": "participating", "万能险": "universal_insurance", "投连险": "investment_linked",
	"养老险": "pension_insurance", "少儿险": "juvenile_insurance", "女性险": "women_insurance",
	"旅游险": "travel_insurance", "留学险": "study_insurance", "签证险": "visa_insurance",
	"延误险": "delay_insurance", "取消险": "cancellation", "行李险": "baggage_insurance",
	"盗抢险": "theft_insurance", "玻璃险": "glass_insurance", "划痕险": "scratch_insurance",
	"自燃险": "spontaneous", "涉水险": "water_damage", "不计免赔": "deductible_waiver",
	"无法找到第三方": "unidentified_third", "指定修理厂": "designated_repair", "代为追偿": "subrogation",
	"道路救援": "roadside_assistance", "拖车": "towing", "搭电": "jump_start",
	"换胎": "tire_change", "送油": "fuel_delivery", "送水": "water_delivery",
	"送钥匙": "key_delivery", "困境救援": "extrication", "吊车": "crane",
	"现场抢修": "on_site_repair", "代步车": "courtesy_car", "酒店住宿": "hotel_accommodation",
	"继续行程": "continue_journey", "返程": "return_trip", "误工": "loss_of_work",
	"住院津贴": "hospital_allowance", "手术津贴": "surgery_allowance", "重症监护": "icu_allowance",
	"特需门诊": "special_clinic", "专家门诊": "expert_clinic", "普通门诊": "general_clinic",
	"急诊": "emergency", "住院": "hospitalization", "门诊": "outpatient",
	"药房": "pharmacy", "药品": "medicine", "处方": "prescription",
	"检查": "examination", "影像": "imaging", "病理": "pathology",
	"手术": "surgery", "麻醉": "anesthesia", "护理": "nursing",
	"康复": "rehabilitation", "疫苗": "vaccine", "接种": "vaccination",
	"防疫": "epidemic_prevention", "隔离": "quarantine", "核酸": "pcr",
	"抗原": "antigen", "抗体": "antibody", "CT": "ct",
	"MRI": "mri", "X光": "xray", "B超": "ultrasound",
	"心电图": "ecg", "脑电图": "eeg", "肌电图": "emg",
	"内窥镜": "endoscopy", "胃镜": "gastroscopy", "肠镜": "colonoscopy",
	"支气管镜": "bronchoscopy", "膀胱镜": "cystoscopy", "宫腔镜": "hysteroscopy",
	"腹腔镜": "laparoscopy", "关节镜": "arthroscopy", "机器人手术": "robotic_surgery",
	"微创": "minimally_invasive", "无创": "non_invasive", "介入": "interventional",
	"放疗": "radiotherapy", "化疗": "chemotherapy", "靶向": "targeted",
	"免疫": "immunotherapy", "基因": "gene", "细胞": "cell",
	"干细胞": "stem_cell", "移植": "transplant", "透析": "dialysis",
	"ICU": "icu", "CCU": "ccu", "NICU": "nicu",
	"PICU": "picu", "急诊室": "er", "抢救室": "resuscitation",
	"手术室": "or", "病房": "ward", "医技": "medical_tech",
	"检验科": "lab", "影像科": "radiology", "病理科": "pathology_dept",
	"药剂科": "pharmacy_dept", "供应室": "sterile_supply", "血库": "blood_bank",
	"病历": "medical_record", "病案": "case_record", "首页": "front_page",
	"病程": "course", "医嘱": "doctor_order", "护理记录": "nursing_record",
	"交接班": "shift_handover", "会诊": "consultation", "转诊": "referral",
	"转科": "transfer_dept", "出院": "discharge", "入院": "admission",
	"挂号": "registration", "预约": "appointment", "排队": "queue",
	"叫号": "call_number", "分诊": "triage", "导诊": "guidance",
	"陪诊": "accompanied", "随访": "follow_up", "纠纷": "dispute",
	"尸检": "autopsy", "医疗事故": "medical_accident", "医疗损害": "medical_damage",
	"医疗过错": "medical_fault", "并发症": "complication", "后遗症": "sequelae",
	"伤残": "disability", "死亡": "death", "死因": "cause_of_death",
	"殡葬": "funeral", "火化": "cremation", "安葬": "burial",
	"墓地": "cemetery", "骨灰": "ashes", "遗体": "remains",
	"器官": "organ", "捐献": "donation", "审查": "review",
	"知情同意": "informed_consent", "隐私": "privacy", "数据安全": "data_security",
	"网络安全": "cyber_security", "等保": "graded_protection", "密评": "cryptographic",
	"关基": "critical_info", "数据出境": "data_export", "个人信息保护": "pipa",
	"数据安全法": "dsl", "网络安全法": "csl", "密码法": "cryptography_law",
	"电子签名法": "e_signature", "电子商务法": "e_commerce", "电子政务": "e_government",
	"数字政府": "digital_government", "智慧城市": "smart_city", "数字乡村": "digital_village",
	"数字经济": "digital_economy", "数字社会": "digital_society", "数字生态": "digital_ecology",
	"数字文明": "digital_civilization", "数字化转型": "digital_transformation", "智能化": "intelligent",
	"自动化": "automation", "信息化": "informatization", "网络化": "networking",
	"云化": "cloudification", "中台化": "platformization", "微服务化": "microservitization",
	"容器化": "containerization", "编排": "orchestrate", "调度": "schedule",
	"伸缩": "scale", "弹性": "elastic", "高可用": "ha",
	"负载均衡": "lb", "故障转移": "failover", "双活": "active_active",
	"主备": "master_slave", "读写分离": "read_write", "分库分表": "sharding",
	"垂直拆分": "vertical", "水平拆分": "horizontal", "冷热分离": "hot_cold",
	"动静分离": "static_dynamic", "CDN": "cdn", "缓存": "cache",
	"预热": "preheat", "刷新": "refresh", "淘汰": "evict",
	"穿透": "penetrate", "击穿": "breakdown", "雪崩": "avalanche",
	"懒加载": "lazy", "预加载": "preload", "异步": "async",
	"同步": "sync", "并发": "concurrent", "并行": "parallel",
	"串行": "serial", "顺序": "sequential", "随机": "random",
	"轮询": "round_robin", "权重": "weighted", "最小连接": "least_conn",
	"哈希": "hash", "一致性": "consistent", "虚拟节点": "virtual_node",
	"槽": "slot", "桶": "bucket", "分区": "partition",
	"分片": "shard", "副本": "replica", "主从": "replication",
	"半同步": "semi_sync", "延迟": "lag", "复制": "replicate",
	"传播": "propagate", "广播": "broadcast", "组播": "multicast",
	"单播": "unicast", "任播": "anycast", "泛洪": "flood",
	"谣言": "rumor", "共识": "consensus", "Paxos": "paxos",
	"Raft": "raft", "ZAB": "zab", "PBFT": "pbft",
	"PoW": "pow", "PoS": "pos", "DPoS": "dpos",
	"BFT": "bft", "CAP": "cap", "BASE": "base",
	"ACID": "acid", "事务": "transaction", "级别": "level",
	"读未提交": "read_uncommitted", "读已提交": "read_committed", "可重复读": "repeatable_read",
	"串行化": "serializable", "脏读": "dirty_read", "幻读": "phantom_read",
	"不可重复读": "non_repeatable", "锁": "lock", "乐观锁": "optimistic",
	"悲观锁": "pessimistic", "共享锁": "shared", "排他锁": "exclusive",
	"意向锁": "intention", "间隙锁": "gap", "临键锁": "next_key",
	"自旋锁": "spin", "读写锁": "rwlock", "信号量": "semaphore",
	"互斥量": "mutex", "条件变量": "condition", "屏障": "barrier",
	"倒计时": "countdown", "门闩": "latch", "令牌桶": "token_bucket",
	"漏桶": "leaky_bucket", "滑动窗口": "sliding_window", "固定窗口": "fixed_window",
	"分布式锁": "dist_lock", "红锁": "redlock", "租约": "lease",
	"会话": "session", "连接": "connection", "池": "pool",
	"线程池": "thread_pool", "协程池": "goroutine_pool", "进程池": "process_pool",
	"对象池": "object_pool", "内存池": "memory_pool", "连接池": "conn_pool",
	"数据库连接池": "db_pool", "资源池": "resource_pool", "工作池": "worker_pool",
	"任务池": "task_pool", "作业池": "job_pool", "批处理": "batch",
	"流处理": "stream", "实时": "realtime", "准实时": "near_realtime",
	"离线": "offline", "近线": "nearline", "在线": "online",
	"OLTP": "oltp", "OLAP": "olap", "HTAP": "htap",
	"湖仓一体": "lakehouse", "数据湖": "datalake", "数据仓库": "datawarehouse",
	"数据中台": "dataplatform", "数据治理": "datagovernance", "数据质量": "dataquality",
	"数据标准": "datastandard", "数据隐私": "dataprivacy", "数据生命周期": "datalifecycle",
	"数据血缘": "datalineage", "数据目录": "datacatalog", "数据资产": "dataasset",
	"数据服务": "dataservice", "数据产品": "dataproduct", "数据运营": "dataops",
	"MLOps": "mlops", "DevOps": "devops", "GitOps": "gitops",
	"FinOps": "finops", "AIOps": "aiops", "BizDevOps": "bizdevops",
	"SRE": "sre", "混沌工程": "chaos", "韧性": "resilience",
	"可靠性": "reliability", "可用性": "availability", "可维护性": "maintainability",
	"可观测性": "observability", "可扩展性": "scalability", "可移植性": "portability",
	"兼容性": "compatibility", "互操作性": "interoperability", "安全性": "security",
	"性能": "performance", "吞吐量": "throughput", "响应时间": "response_time",
	"并发数": "concurrency", "QPS": "qps", "TPS": "tps",
	"RPS": "rps", "RPM": "rpm", "PV": "pv",
	"UV": "uv", "DAU": "dau", "MAU": "mau",
	"WAU": "wau", "留存率": "retention_rate", "活跃率": "active_rate",
	"转化率": "conversion_rate", "点击率": "ctr", "曝光率": "impression_rate",
	"到达率": "reach_rate", "完播率": "completion_rate", "跳出率": "bounce_rate",
	"退出率": "exit_rate", "平均停留": "avg_duration", "深度": "depth",
	"频次": "frequency", "周期": "period", "时长": "duration",
	"间隔": "interval", "超时": "timeout", "过期": "expire",
	"作废": "void", "撤销": "revoke", "取消": "cancel",
	"终止": "terminate", "暂停": "pause", "恢复": "resume",
	"继续": "continue", "重试": "retry", "回滚": "rollback",
	"重做": "redo", "历史": "history", "版本": "version",
	"快照": "snapshot", "镜像": "mirror", "克隆": "clone",
	"备份": "backup", "归档": "archive", "导出": "export",
	"导入": "import", "上传": "upload", "下载": "download",
	"推送": "push", "拉取": "pull", "发布": "publish",
	"订阅": "subscribe", "取消订阅": "unsubscribe", "取消关注": "unfollow",
	"取消收藏": "unfavorite", "取消点赞": "unlike", "回复": "reply",
	"转发": "forward", "举报": "report", "屏蔽": "block",
	"拉黑": "blacklist", "白名单": "whitelist", "灰名单": "graylist",
	"名单": "list", "群组": "group", "社区": "community",
	"问答": "qa", "百科": "wiki", "词典": "dictionary",
	"术语": "term", "定义": "definition", "标准": "standard",
	"规范": "specification", "制度": "policy", "指引": "guideline",
	"手册": "manual", "指南": "guide", "教程": "tutorial",
	"资料": "material", "档案": "archive", "记录": "record",
	"变更": "change", "部署": "deploy", "灰度": "gray",
	"迁移": "migrate", "集群": "cluster", "节点": "node",
	"实例": "instance", "应用": "app", "系统": "system",
	"模块": "module", "组件": "component", "接口": "interface",
	"API": "api", "SDK": "sdk", "包": "package",
	"库": "library", "框架": "framework", "引擎": "engine",
	"平台": "platform", "中台": "platform", "底座": "base",
	"基础": "base", "核心": "core", "公共": "common",
	"通用": "common", "共享": "share", "统一": "unified",
	"约定": "convention", "对接": "dock", "集成": "integrate",
	"联通": "connect", "打通": "connect", "闭环": "loop",
	"链路": "chain", "路径": "path", "路由": "route",
	"网关": "gateway", "负载": "load", "均衡": "balance",
	"限流": "limit", "熔断": "circuit", "降级": "degrade",
	"容灾": "disaster", "监控": "monitor", "告警": "alert",
	"预警": "warn", "消费": "consume", "队列": "queue",
	"主题": "topic", "索引": "index", "主键": "pk",
	"外键": "fk", "约束": "constraint", "触发器": "trigger",
	"存储过程": "procedure", "函数": "function", "视图": "view",
	"物化视图": "mview", "临时表": "temp", "清理": "purge",
	"压缩": "compress", "加密": "encrypt", "签名": "sign",
	"验签": "verify", "令牌": "token", "票据": "ticket",
	"证书": "certificate", "密钥": "key", "密码": "password",
	"口令": "password", "授权": "authorize", "鉴权": "auth",
	"登录": "login", "登出": "logout", "激活": "activate",
	"禁用": "disable", "启用": "enable", "锁定": "lock",
	"解锁": "unlock", "冻结": "freeze", "解冻": "unfreeze",
	"注销": "cancel", "销户": "close", "开户": "open",
	"绑卡": "bind", "解绑": "unbind", "换绑": "rebind",
	"实名": "realname", "身份": "identity", "证件": "idcard",
	"身份证": "idcard", "驾照": "license", "营业执照": "license",
	"组织": "org", "机构": "org", "团队": "team",
	"小组": "group", "编制": "establishment", "人力": "hr",
	"人事": "hr", "行政": "admin", "后勤": "logistics",
	"IT": "it", "数字化": "digital", "转型": "transform",
	"改造": "renovate", "优化": "optimize", "提升": "improve",
	"降低": "reduce", "控制": "control", "管理": "manage",
	"治理": "governance", "监管": "regulate", "监督": "supervise",
	"稽核": "audit", "内控": "internal_control", "风控": "risk_control",
	"法务": "legal", "报价": "quote", "询价": "inquiry",
	"招标": "bid", "投标": "tender", "中标": "win",
	"签约": "sign", "履约": "perform", "结算": "settle",
	"付款": "pay", "收款": "receive", "开票": "invoice",
	"收票": "receive_invoice", "对账": "reconcile", "预收": "pre_receive",
	"预付": "pre_pay", "应收": "receivable", "应付": "payable",
	"预提": "accrual", "摊销": "amortize", "折旧": "depreciate",
	"减值": "impair", "公允": "fair", "价值": "value",
	"评估": "appraise", "估值": "valuation", "测算": "calculate",
	"决算": "final", "概算": "estimate", "估算": "estimate",
	"预测": "forecast", "实际": "actual", "差异": "variance",
	"分析": "analysis", "归因": "attribution", "洞察": "insight",
	"发现": "find", "趋势": "trend", "波动": "fluctuation",
	"季节": "seasonal", "同比": "yoy", "环比": "mom",
	"增长率": "growth", "占比": "ratio", "贡献": "contribution",
	"结构": "structure", "分布": "distribution", "集中度": "concentration",
	"渗透率": "penetration", "覆盖率": "coverage", "留存": "retention",
	"流失": "churn", "活跃": "active", "沉睡": "dormant",
	"唤醒": "awaken", "召回": "recall", "转化": "convert",
	"漏斗": "funnel", "触点": "touchpoint", "旅程": "journey",
	"体验": "experience", "感知": "perception", "期望": "expectation",
	"需求": "need", "痛点": "pain", "场景": "scenario",
	"分群": "segment", "分层": "tier", "分级": "grade",
	"分档": "band", "特征": "feature", "因子": "factor",
	"得分": "score", "评级": "rating", "排名": "rank",
	"排序": "sort", "匹配": "match", "相似": "similar",
	"关联": "associate", "过滤": "filter", "算法": "algorithm",
	"训练": "train", "聚类": "cluster", "回归": "regress",
	"时序": "time_series", "识别": "recognize", "抽取": "extract",
	"解析": "parse", "理解": "understand", "生成": "generate",
	"合成": "synthesize", "增强": "augment", "采样": "sample",
	"降维": "reduce", "选择": "select", "工程": "engineer",
	"调参": "tune", "网格": "grid", "贝叶斯": "bayesian",
	"进化": "evolution", "遗传": "genetic", "强化": "reinforcement",
	"联邦": "federated", "差分": "differential", "防护": "protect",
	"攻击": "attack", "漏洞": "vulnerability", "威胁": "threat",
	"量化": "quantify", "建模": "model", "仿真": "simulate",
	"演练": "drill", "应急": "emergency", "处置": "dispose",
	"重建": "rebuild", "保留": "retain", "销毁": "destroy",
	"脱敏": "desensitize", "匿名": "anonymize", "假名": "pseudonym",
	"令牌化": "tokenize", "加盐": "salt", "迭代": "iterate",
	"次数": "count", "强度": "strength", "复杂度": "complexity",
	"熵": "entropy", "种子": "seed", "序列": "sequence",
	"流水": "serial", "条码": "barcode", "二维码": "qrcode",
	"RFID": "rfid", "NFC": "nfc", "芯片": "chip",
	"传感器": "sensor", "物联网": "iot", "边缘": "edge",
	"云端": "cloud", "雾计算": "fog", "区块链": "blockchain",
	"分布式": "distributed", "去中心": "decentralized", "智能合约": "contract",
	"代币": "token", "NFT": "nft", "元宇宙": "metaverse",
	"数字孪生": "digital_twin", "虚拟现实": "vr", "增强现实": "ar",
	"混合现实": "mr", "全息": "hologram", "3D": "3d",
	"渲染": "render", "数字人": "digital_human", "虚拟人": "virtual_human",
	"机器人": "robot", "RPA": "rpa", "低代码": "lowcode",
	"无代码": "nocode", "拖拉拽": "drag_drop", "配置化": "configurable",
	"参数化": "parametric", "模板化": "templatized", "标准化": "standardized",
	"规范化": "normalized", "模块化": "modular", "组件化": "componentized",
	"服务化": "servitized", "微服务": "microservice", "DevOps化": "devopsification",
	"DataOps化": "dataopsification", "MLOps化": "mlopsification", "AIOps化": "aiopsification",
	"FinOps化": "finopsification", "BizDevOps化": "bizdevopsification", "SRE化": "sreification",
	"混沌工程化": "chaos_engineering", "韧性工程": "resilience_engineering", "可靠性工程": "reliability_engineering",
	"可观测性工程": "observability_engineering", "性能工程": "performance_engineering", "安全工程": "security_engineering",
	"隐私工程": "privacy_engineering", "合规工程": "compliance_engineering", "治理工程": "governance_engineering",
	"数据工程": "data_engineering", "AI工程": "ai_engineering", "ML工程": "ml_engineering",
	"DL工程": "dl_engineering", "NLP工程": "nlp_engineering", "CV工程": "cv_engineering",
	"语音工程": "speech_engineering", "推荐工程": "recommendation_engineering", "搜索工程": "search_engineering",
	"广告工程": "ad_engineering", "风控工程": "risk_engineering", "反欺诈工程": "anti_fraud_engineering",
	"反洗钱工程": "anti_money_laundering", "合规科技": "regtech", "监管科技": "suptech",
	"保险科技": "insurtech", "财富科技": "wealthtech", "信贷科技": "lendtech",
	"支付科技": "paytech", "区块链科技": "blockchain_tech", "物联网科技": "iot_tech",
	"边缘科技": "edge_tech", "量子科技": "quantum_tech", "生物科技": "bio_tech",
	"基因科技": "gene_tech", "纳米科技": "nano_tech", "新材料科技": "new_material",
	"新能源科技": "new_energy", "清洁能源": "clean_energy", "可再生能源": "renewable_energy",
	"核能": "nuclear", "风能": "wind", "太阳能": "solar",
	"水能": "hydro", "地热能": "geothermal", "生物质能": "biomass",
	"氢能": "hydrogen", "储能": "energy_storage", "电池": "battery",
	"充电桩": "charging_pile", "电网": "power_grid", "变电站": "substation",
	"输电": "transmission", "配电": "distribution", "用电": "consumption",
	"售电": "retail_electricity", "电力交易": "power_trading", "碳交易": "carbon_trading",
	"绿证": "green_certificate", "碳汇": "carbon_sink", "碳捕集": "ccs",
	"碳利用": "ccu", "碳封存": "ccus", "DAC": "dac",
	"BECCS": "beccs", "直接空气捕集": "direct_air_capture", "生物能源碳捕集": "bioenergy_ccs",
	"海洋碳汇": "ocean_sink", "森林碳汇": "forest_sink", "土壤碳汇": "soil_sink",
	"湿地碳汇": "wetland_sink", "草原碳汇": "grassland_sink", "农田碳汇": "farmland_sink",
	"城市碳汇": "urban_sink", "蓝碳": "blue_carbon", "绿碳": "green_carbon",
	"棕碳": "brown_carbon", "黑碳": "black_carbon", "碳足迹": "carbon_footprint",
	"水足迹": "water_footprint", "生态足迹": "ecological_footprint", "环境足迹": "environmental_footprint",
	"产品足迹": "product_footprint", "组织足迹": "organization_footprint", "项目足迹": "project_footprint",
	"供应链足迹": "supply_chain_footprint", "价值链足迹": "value_chain_footprint", "生命周期": "life_cycle",
	"LCA": "lca", "EPD": "epd", "EPR": "epr",
	"PFAS": "pfas", "POPs": "pops", "VOCs": "vocs",
	"NOx": "nox", "SOx": "sox", "PM2.5": "pm25",
	"PM10": "pm10", "臭氧": "o3", "温室气体": "ghg",
	"CO2": "co2", "CH4": "ch4", "N2O": "n2o",
	"HFCs": "hfcs", "PFCs": "pfcs", "SF6": "sf6",
	"NF3": "nf3", "ODS": "ods", "CFCs": "cfcs",
	"HCFCs": "hcfcs", "甲基溴": "methyl_bromide", "四氯化碳": "carbon_tetrachloride",
	"哈龙": "halon", "消耗臭氧层": "ozone_depleting", "蒙特利尔": "montreal",
	"基加利": "kigali", "巴黎协定": "paris", "京都议定书": "kyoto",
	"哥本哈根": "copenhagen", "坎昆": "cancun", "德班": "durban",
	"多哈": "doha", "利马": "lima", "华沙": "warsaw",
	"马拉喀什": "marrakech", "波恩": "bonn", "波兹南": "poznan",
	"巴厘岛": "bali", "内罗毕": "nairobi", "布宜诺斯": "buenos_aires",
	"海牙": "hague", "柏林": "berlin", "日内瓦": "geneva",
	"维也纳": "vienna", "纽约": "new_york", "伦敦": "london",
	"东京": "tokyo", "悉尼": "sydney", "新加坡": "singapore",
	"迪拜": "dubai", "孟买": "mumbai", "班加罗尔": "bangalore",
	"上海": "shanghai", "北京": "beijing", "深圳": "shenzhen",
	"广州": "guangzhou", "杭州": "hangzhou", "成都": "chengdu",
	"武汉": "wuhan", "西安": "xian", "南京": "nanjing",
	"苏州": "suzhou", "天津": "tianjin", "重庆": "chongqing",
	"青岛": "qingdao", "大连": "dalian", "宁波": "ningbo",
	"厦门": "xiamen", "无锡": "wuxi", "佛山": "foshan",
	"东莞": "dongguan", "长沙": "changsha", "郑州": "zhengzhou",
	"福州": "fuzhou", "合肥": "hefei", "济南": "jinan",
	"昆明": "kunming", "沈阳": "shenyang", "哈尔滨": "harbin",
	"长春": "changchun", "石家庄": "shijiazhuang", "太原": "taiyuan",
	"南昌": "nanchang", "贵阳": "guiyang", "南宁": "nanning",
	"兰州": "lanzhou", "海口": "haikou", "银川": "yinchuan",
	"西宁": "xining", "拉萨": "lhasa", "乌鲁木齐": "urumqi",
	"呼和浩特": "hohhot", "台北": "taipei", "香港": "hong_kong",
	"澳门": "macau", "大阪": "osaka", "名古屋": "nagoya",
	"福冈": "fukuoka", "札幌": "sapporo", "仙台": "sendai",
	"广岛": "hiroshima", "京都": "kyoto", "神户": "kobe",
	"横滨": "yokohama", "千叶": "chiba", "埼玉": "saitama",
	"兵库": "hyogo", "奈良": "nara", "滋贺": "shiga",
	"三重": "mie", "岐阜": "gifu", "静冈": "shizuoka",
	"爱知": "aichi", "新潟": "niigata", "富山": "toyama",
	"石川": "ishikawa", "福井": "fukui", "山梨": "yamanashi",
	"长野": "nagano", "群马": "gunma", "栃木": "tochigi",
	"茨城": "ibaraki", "福岛": "fukushima", "宫城": "miyagi",
	"岩手": "iwate", "秋田": "akita", "山形": "yamagata",
	"青森": "aomori", "北海道": "hokkaido", "冲绳": "okinawa",
	"鹿儿岛": "kagoshima", "宫崎": "miyazaki", "大分": "oita",
	"熊本": "kumamoto", "长崎": "nagasaki", "佐贺": "saga",
	"山口": "yamaguchi", "岛根": "shimane", "鸟取": "tottori",
	"冈山": "okayama", "香川": "kagawa", "爱媛": "ehime",
	"德岛": "tokushima", "高知": "kochi", "首尔": "seoul",
	"釜山": "busan", "仁川": "incheon", "大邱": "daegu",
	"大田": "daejeon", "光州": "gwangju", "蔚山": "ulsan",
	"世宗": "sejong", "曼谷": "bangkok", "吉隆坡": "kuala_lumpur",
	"雅加达": "jakarta", "马尼拉": "manila", "河内": "hanoi",
	"胡志明": "ho_chi_minh", "金边": "phnom_penh", "万象": "vientiane",
	"仰光": "yangon", "达卡": "dhaka", "新德里": "new_delhi",
	"加尔各答": "kolkata", "钦奈": "chennai", "海得拉巴": "hyderabad",
	"浦那": "pune", "艾哈迈达": "ahmedabad", "苏拉特": "surat",
	"斋浦尔": "jaipur", "勒克瑙": "lucknow", "坎普尔": "kanpur",
	"那格浦尔": "nagpur", "印多尔": "indore", "塔那": "thane",
	"博帕尔": "bhopal", "维沙卡帕": "visakhapatnam", "巴特那": "patna",
	"卢迪亚纳": "ludhiana", "阿格拉": "agra", "纳西克": "nashik",
	"法里达巴": "faridabad", "密拉特": "meerut", "拉杰科特": "rajkot",
	"瓦拉纳西": "varanasi", "斯利那加": "srinagar", "奥兰加巴": "aurangabad",
	"丹巴德": "dhanbad", "安拉阿巴": "allahabad", "兰契": "ranchi",
	"豪拉": "howrah", "科钦": "cochin", "本地治里": "pondicherry",
	"特里凡得": "trivandrum", "科泽科德": "calicut", "哥印拜陀": "coimbatore",
	"马杜赖": "madurai", "特里奇": "trichy", "塞勒姆": "salem",
	"蒂鲁伯尔": "tiruppur", "埃罗德": "erode", "韦洛尔": "vellore",
	"蒂鲁内尔": "tirunelveli", "纳盖科伊尔": "nagercoil", "甘吉布勒": "cuddalore",
	"阿布扎比": "abu_dhabi", "沙迦": "sharjah", "阿治曼": "ajman",
	"乌姆盖万": "umm_al_quwain", "富查伊拉": "fujairah", "麦纳麦": "manama",
	"科威特": "kuwait_city", "利雅得": "riyadh", "吉达": "jeddah",
	"麦加": "mecca", "麦地那": "medina", "达曼": "dammam",
	"塔布克": "tabuk", "哈伊勒": "hail", "阿西尔": "asir",
	"巴哈": "baha", "焦夫": "jawf", "北部边疆": "northern_border",
	"东部": "eastern", "卡西姆": "qassim", "利雅得省": "riyadh_province",
	"麦加省": "mecca_province", "麦地那省": "medina_province", "东部省": "eastern_province",
	"阿西尔省": "asir_province", "巴哈省": "baha_province", "焦夫省": "jawf_province",
	"北部边疆省": "northern_border_province", "塔布克省": "tabuk_province", "哈伊勒省": "hail_province",
	"奈季兰省": "najran_province", "吉赞省": "jazan_province", "卡西姆省": "qassim_province",
	"马斯喀特": "muscat", "塞拉莱": "salalah", "苏哈尔": "sohar",
	"尼兹瓦": "nizwa", "伊布拉": "ibra", "巴赫拉": "bahla",
	"鲁斯塔克": "rustaq", "苏维克": "suwaiq", "希纳斯": "shinas",
	"海塞卜": "khasab", "德黑兰": "tehran", "伊斯法罕": "isfahan",
	"马什哈德": "mashhad", "大不里士": "tabriz", "设拉子": "shiraz",
	"阿瓦士": "ahvaz", "克尔曼": "kerman", "亚兹德": "yazd",
	"拉什特": "rasht", "萨里": "sari", "加兹温": "qazvin",
	"赞詹": "zanjan", "阿尔达比": "ardabil", "乌尔米耶": "urmia",
	"克尔曼沙": "kermanshah", "哈马丹": "hamadan", "萨南达季": "sanandaj",
	"伊拉姆": "ilam", "霍拉马巴": "khorramabad", "博鲁杰尔": "borujerd",
	"迪兹富勒": "dezful", "舒什塔尔": "shushtar", "阿巴丹": "abadan",
	"霍拉姆沙": "khorramshahr", "布什尔": "bushehr", "阿萨卢耶": "asaluyeh",
	"班达尔阿": "bandar_abbas", "阿巴斯港": "bandar_abbas", "恰巴哈尔": "chabahar",
	"扎黑丹": "zahedan", "比尔詹德": "birjand", "内沙布尔": "neyshabur",
	"萨卜泽瓦": "sabzevar", "托尔巴特": "torbat", "贡巴德": "gonbad",
	"戈尔甘": "gorgan", "巴博勒": "babol", "阿莫勒": "amol",
	"恰卢斯": "chalus", "瑙沙赫尔": "nowshahr",
}

func (r *EntityRecognizer) ExtractTableNames(text string) []string {
	var tables []string
	lower := strings.ToLower(text)
	for name := range r.kb.GetAllTables() {
		nameLower := strings.ToLower(name)
		if strings.Contains(lower, nameLower) {
			tables = append(tables, name)
			continue
		}
		parts := strings.Split(name, "_")
		if len(parts) >= 2 {
			allPartsFound := true
			for _, part := range parts {
				if len(part) >= 2 && !strings.Contains(lower, strings.ToLower(part)) {
					allPartsFound = false
					break
				}
			}
			if allPartsFound {
				tables = append(tables, name)
			}
		}
	}
	return tables
}

func (r *EntityRecognizer) Recognize(text string) *RecognizedInput {
	result := &RecognizedInput{
		RawText: text,
		Fields:  make(map[string][]string),
	}

	result.Tables = r.extractTables(text)
	result.Conditions = r.extractConditions(text)
	result.Intent = r.detectIntent(text)

	for _, table := range result.Tables {
		t := r.kb.GetTable(table)
		if t != nil {
			for _, col := range t.ColumnsJSON {
				if strings.Contains(text, col.Name) || strings.Contains(text, col.Comment) {
					result.Fields[table] = append(result.Fields[table], col.Name)
				}
			}
		}
	}

	return result
}

func (r *EntityRecognizer) extractTables(text string) []string {
	var tables []string
	seen := make(map[string]bool)

	textLower := strings.ToLower(text)

	// 1. 精确匹配：表名直接出现在文本中
	for name := range r.kb.GetAllTables() {
		if strings.Contains(textLower, strings.ToLower(name)) {
			if !seen[name] {
				tables = append(tables, name)
				seen[name] = true
			}
		}
	}

	// 2. 表注释匹配
	for name, cache := range r.kb.GetAllTables() {
		if cache.TableComment != "" && strings.Contains(textLower, strings.ToLower(cache.TableComment)) {
			if !seen[name] {
				tables = append(tables, name)
				seen[name] = true
			}
		}
	}

	// 3. 业务关键词模糊匹配
	// 提取文本中的中文字符串，尝试匹配关键词映射
	chineseWords := extractChineseWords(text)
	for _, word := range chineseWords {
		if englishWord, ok := bizKeywordMap[word]; ok {
			// 在表名中查找是否包含这个英文词
			for name := range r.kb.GetAllTables() {
				if seen[name] {
					continue
				}
				nameLower := strings.ToLower(name)
				// 去掉常见前缀如 t_, tb_, tbl_ 后匹配
				cleanName := stripTablePrefix(nameLower)
				if strings.Contains(cleanName, englishWord) || strings.Contains(nameLower, englishWord) {
					tables = append(tables, name)
					seen[name] = true
				}
			}
		}
	}

	// 4. 列名反向推断：如果文本中提到了某列，找到包含该列的表
	for name, cache := range r.kb.GetAllTables() {
		if seen[name] {
			continue
		}
		for _, col := range cache.ColumnsJSON {
			if col.Comment != "" && strings.Contains(textLower, strings.ToLower(col.Comment)) {
				tables = append(tables, name)
				seen[name] = true
				break
			}
			if col.Name != "" && strings.Contains(textLower, strings.ToLower(col.Name)) {
				tables = append(tables, name)
				seen[name] = true
				break
			}
		}
	}

	return tables
}

// extractChineseWords 从文本中提取连续的中文字符串
func extractChineseWords(text string) []string {
	var words []string
	var current strings.Builder

	for _, r := range text {
		if unicode.Is(unicode.Han, r) {
			current.WriteRune(r)
		} else {
			if current.Len() > 0 {
				words = append(words, current.String())
				current.Reset()
			}
		}
	}
	if current.Len() > 0 {
		words = append(words, current.String())
	}

	// 同时提取2-4个字的子串，用于匹配复合词
	var subWords []string
	for _, word := range words {
		runes := []rune(word)
		for length := 2; length <= 4 && length <= len(runes); length++ {
			for i := 0; i <= len(runes)-length; i++ {
				subWords = append(subWords, string(runes[i:i+length]))
			}
		}
	}

	return append(words, subWords...)
}

// stripTablePrefix 去掉表名常见前缀
func stripTablePrefix(name string) string {
	prefixes := []string{"t_", "tb_", "tbl_", "table_", "m_", "aw_", "sys_"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(name, prefix) {
			return name[len(prefix):]
		}
	}
	return name
}

func (r *EntityRecognizer) extractConditions(text string) []Condition {
	var conditions []Condition

	patterns := []struct {
		regex *regexp.Regexp
		op    string
	}{
		{regexp.MustCompile(`(\w+)\s*(?:等于|=|是)\s*['"]?([^'"\s]+)['"]?`), "="},
		{regexp.MustCompile(`(\w+)\s*(?:大于|>)\s*(\d+)`), ">"},
		{regexp.MustCompile(`(\w+)\s*(?:小于|<)\s*(\d+)`), "<"},
		{regexp.MustCompile(`(\w+)\s*(?:大于等于|>=)\s*(\d+)`), ">="},
		{regexp.MustCompile(`(\w+)\s*(?:小于等于|<=)\s*(\d+)`), "<="},
		{regexp.MustCompile(`(\w+)\s*(?:不等于|!=|<>)\s*['"]?([^'"\s]+)['"]?`), "!="},
		{regexp.MustCompile(`(\w+)\s*(?:包含|含有|like)\s*['"]?([^'"\s]+)['"]?`), "LIKE"},
	}

	for _, p := range patterns {
		matches := p.regex.FindAllStringSubmatch(text, -1)
		for _, m := range matches {
			if len(m) >= 3 {
				conditions = append(conditions, Condition{
					Field: m[1],
					Op:    p.op,
					Value: m[2],
				})
			}
		}
	}

	return conditions
}

func (r *EntityRecognizer) detectIntent(text string) string {
	text = strings.ToLower(text)

	selectKeywords := []string{"查询", "查找", "获取", "列出", "显示", "捞取", "查", "找", "select", "query"}
	insertKeywords := []string{"插入", "新增", "添加", "创建", "insert", "create"}
	updateKeywords := []string{"更新", "修改", "update"}
	deleteKeywords := []string{"删除", "移除", "delete", "remove"}
	countKeywords := []string{"统计", "计数", "数量", "count"}

	for _, kw := range selectKeywords {
		if strings.Contains(text, kw) {
			return "SELECT"
		}
	}
	for _, kw := range countKeywords {
		if strings.Contains(text, kw) {
			return "COUNT"
		}
	}
	for _, kw := range insertKeywords {
		if strings.Contains(text, kw) {
			return "INSERT"
		}
	}
	for _, kw := range updateKeywords {
		if strings.Contains(text, kw) {
			return "UPDATE"
		}
	}
	for _, kw := range deleteKeywords {
		if strings.Contains(text, kw) {
			return "DELETE"
		}
	}

	return "SELECT"
}
