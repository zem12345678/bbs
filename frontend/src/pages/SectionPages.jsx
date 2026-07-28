import React from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import {
  Activity,
  BadgePercent,
  CalendarDays,
  CircleHelp,
  Compass,
  ExternalLink,
  FileText,
  FolderOpen,
  Grid3X3,
  MessageCircle,
  Rocket,
  Search,
  ShieldCheck,
  ShoppingBag,
  SlidersHorizontal,
  Star,
  Trophy,
  Users,
  X
} from "lucide-react";
import { bbsApi } from "../api";
import { listItems, listTotal } from "../lib/apiShapes";
import { safeExternalURL } from "../lib/externalLinks.js";
import { loadAllListPages } from "../lib/focusedLists";
import { timeAgoMillis, toId, toNumber } from "../lib/formatters";
import { checkoutAttemptKey, checkoutAttemptOrderIds, clearCheckoutAttemptKey, paymentAttemptKey, recordCheckoutAttemptOrder } from "../lib/idempotencyKeys";
import { MALL_COUPON_CHECKOUT_STATUS, mallCouponCheckoutMessage, mallCouponCheckoutState, mallCouponIsAvailable, shouldBlockMallCheckoutForBalance } from "../lib/mallCoupons";
import { friendlyMallCheckoutError, friendlyMallReviewError, shouldRefreshMallCouponsAfterError, shouldRefreshMallInventoryAfterError } from "../lib/mallErrors";
import { mallOrderCanPay, mallOrderPaymentSettled, mallOrderStatusLabel } from "../lib/mallOrders";
import { mallGrantKeyOf, mallGrantLabel, mallGrantSnapshotText, mallGrantTypeOf, mallProductRequiresShipping, parseShopDeepLink, sortProductsForStorefront } from "../lib/mallProducts";
import { appendMarkdownImage, markdownImageUrls, textWithoutMarkdownImages } from "../lib/markdownMedia";
import { EmptyState } from "./RouteBlocks.jsx";
import {
  BlockHeader,
  CircleCard,
  ListRow,
  MetricCard,
  MoreCard,
  PageHero,
  ProductCard,
  QuestionCard,
  ResourceCard,
  StepItem,
  TrendBar
} from "./SectionBlocks.jsx";
import { pageImages, workspacePhotos } from "./sectionData";

const COUPON_USAGE_STATUS_CLAIMED = 4;
const SHOP_PRODUCT_PAGE_SIZE = 24;
const SHOP_CATEGORY_PAGE_SIZE = 100;
const SHOP_PRODUCT_REVIEW_PAGE_SIZE = 10;
const SHOP_REVIEWABLE_ORDER_PAGE_SIZE = 20;
const SHOP_COUPON_PAGE_SIZE = 12;
const SHOP_FAVORITE_PAGE_SIZE = 20;
const SHOP_ADDRESS_PAGE_SIZE = 20;
const HELP_QUESTION_PAGE_SIZE = 8;
const RESOURCE_PAGE_SIZE = 12;

export function HomePage({ categories = [], hotTags = [] }) {
  const [reloadKey, setReloadKey] = React.useState(0);
  const [state, setState] = React.useState({
    topics: [],
    articles: [],
    loading: true,
    error: ""
  });

  React.useEffect(() => {
    let alive = true;
    setState((current) => ({ ...current, loading: true, error: "" }));
    Promise.allSettled([
      bbsApi.listTopics({ limit: 8, offset: 0 }),
      bbsApi.listArticles({ limit: 8, offset: 0 })
    ]).then(([topicResult, articleResult]) => {
      if (!alive) return;
      const topics = topicResult.status === "fulfilled" ? listItems(topicResult.value) : [];
      const articles = articleResult.status === "fulfilled" ? listItems(articleResult.value) : [];
      const failed = topicResult.status === "rejected" && articleResult.status === "rejected";
      setState({
        topics,
        articles,
        loading: false,
        error: failed ? "首页数据加载失败，请稍后重试。" : ""
      });
    });
    return () => {
      alive = false;
    };
  }, [reloadKey]);

  const recentItems = [...state.topics.map((item) => homeContentItem(item, "话题")), ...state.articles.map((item) => homeContentItem(item, "文章"))]
    .sort((a, b) => b.sortAt - a.sortAt)
    .slice(0, 4);
  const recentCount = state.topics.length + state.articles.length;
  const metricCards = [
    {
      label: "近期话题",
      value: state.loading ? "..." : String(state.topics.length),
      meta: state.error ? "加载失败" : state.topics.length > 0 ? "实时更新" : "暂无新话题",
      icon: MessageCircle
    },
    {
      label: "近期文章",
      value: state.loading ? "..." : String(state.articles.length),
      meta: state.error ? "加载失败" : state.articles.length > 0 ? "实时更新" : "暂无新文章",
      icon: FileText
    },
    {
      label: "开放分类",
      value: String(categories.length),
      meta: categories.length > 0 ? "已开放" : "暂无分类",
      icon: Users
    },
    {
      label: "热门标签",
      value: String(hotTags.length),
      meta: hotTags.length > 0 ? "持续更新" : "暂无标签",
      icon: FolderOpen
    }
  ];

  return (
    <>
      <PageHero
        icon={Compass}
        eyebrow="社区首页"
        title="今天的社区协作都在这里"
        description="快速查看关注动态、待办求助、热门圈子和资源更新，把分散的信息收进一个工作台。"
        image={pageImages.首页}
        stats={[
          [String(categories.length), "开放分类"],
          [state.loading ? "..." : String(recentCount), "近期内容"],
          [String(hotTags.length), "热门标签"]
        ]}
      />
      <div className="metric-grid">
        {metricCards.map((item) => (
          <MetricCard key={item.label} item={item} />
        ))}
      </div>
      <section className="panel content-block">
        <BlockHeader icon={CalendarDays} title="最新社区内容" action="刷新" onAction={() => setReloadKey((value) => value + 1)} />
        <div className="timeline-list">
          {state.loading && <ListRow title="正在加载社区内容" meta="请稍候" />}
          {!state.loading && state.error && <ListRow title={state.error} meta="可以点击刷新重新加载" />}
          {!state.loading && !state.error && recentItems.length === 0 && <ListRow title="暂无社区内容" meta="发布第一条话题或文章后会显示在这里" />}
          {!state.loading && !state.error && recentItems.map((item) => (
            <div className="timeline-item" key={`${item.type}-${item.id}`}>
              <time>{item.time}</time>
              <div>
                <strong>{item.title}</strong>
                <p>{item.desc}</p>
              </div>
            </div>
          ))}
        </div>
      </section>
    </>
  );
}

export function CirclesPage({ categories = [], hotTags = [] }) {
  const navigate = useNavigate();
  const totalTopics = categories.reduce((sum, item) => sum + toNumber(item.topicCount), 0);

  return (
    <>
      <PageHero
        icon={Users}
        eyebrow="圈子"
        title="按分类浏览社区讨论"
        description="每个分类围绕稳定主题沉淀帖子、资源和实践经验，便于快速定位讨论。"
        image={pageImages.圈子}
        stats={[
          [String(categories.length), "开放分类"],
          [String(hotTags.length), "热门标签"],
          [String(totalTopics), "已发布话题"]
        ]}
      />
      {categories.length === 0 ? (
        <EmptyState title="暂无分类数据" description="分类开放后会显示在这里。" />
      ) : (
        <div className="circle-grid">
          {categories.map((category) => (
            <CircleCard category={category} key={category.id || category.name} />
          ))}
        </div>
      )}
      <section className="panel content-block">
        <BlockHeader icon={Rocket} title="分类概览" action="查看全部话题" onAction={() => navigate("/topics")} />
        <div className="compact-list">
          {categories.length === 0 && <ListRow title="暂无分类概览" meta="发布内容并维护分类后会显示更新" />}
          {categories.slice(0, 4).map((category) => (
            <ListRow
              key={category.id || category.name}
              title={category.name}
              meta={`${category.topicCountKnown ? `${category.topicCount} 条话题` : "话题数统计中"} · ${category.description || "暂无分类说明"}`}
            />
          ))}
        </div>
      </section>
    </>
  );
}

export function HelpPage() {
  const navigate = useNavigate();
  const [state, setState] = React.useState({ items: [], total: 0, offset: 0, loading: true, loadingMore: false, error: "" });

  const loadQuestions = React.useCallback((offset = 0, appending = false) => {
    let alive = true;
    setState((current) => ({
      ...current,
      items: appending ? current.items : [],
      total: appending ? current.total : 0,
      offset: appending ? current.offset : 0,
      loading: !appending,
      loadingMore: appending,
      error: ""
    }));
    bbsApi
      .listTopics({ type: "qa", limit: HELP_QUESTION_PAGE_SIZE, offset })
      .then((data) => {
        if (!alive) return;
        const pageItems = listItems(data);
        if (appending) {
          setState((current) => {
            const keys = new Set(current.items.map((item) => String(item?.id ?? "")));
            const items = [
              ...current.items,
              ...pageItems.filter((item) => {
                const key = String(item?.id ?? "");
                if (keys.has(key)) return false;
                keys.add(key);
                return true;
              })
            ];
            const nextOffset = current.offset + pageItems.length;
            return {
              ...current,
              items,
              total: pageItems.length > 0 ? Math.max(nextOffset, listTotal(data, pageItems)) : current.offset,
              offset: nextOffset,
              loadingMore: false,
              error: ""
            };
          });
          return;
        }
        setState({
          items: pageItems,
          total: Math.max(listTotal(data, pageItems), pageItems.length),
          offset: pageItems.length,
          loading: false,
          loadingMore: false,
          error: ""
        });
      })
      .catch((error) => {
        if (!alive) return;
        if (appending) {
          setState((current) => ({ ...current, loadingMore: false, error: error.message || "更多求助内容加载失败" }));
          return;
        }
        setState({ items: [], total: 0, offset: 0, loading: false, loadingMore: false, error: error.message || "求助内容加载失败" });
      });
    return () => {
      alive = false;
    };
  }, []);

  React.useEffect(loadQuestions, [loadQuestions]);

  function loadMoreQuestions() {
    if (state.loading || state.loadingMore || state.offset >= state.total) return;
    loadQuestions(state.offset, true);
  }

  const questions = state.items.map(topicToQuestion);

  return (
    <>
      <PageHero
        icon={CircleHelp}
        eyebrow="求助"
        title="把问题说清楚，让答案更快到达"
        description="求助区按状态、悬赏和标签组织问题，适合追踪排障进展和沉淀可复用解法。"
        image={pageImages.求助}
        stats={[
          [state.loading ? "..." : String(state.total), "可参与讨论"],
          [state.loading ? "..." : String(questions.filter((item) => item.answers > 0).length), "已有回复"],
          [state.error ? "失败" : "实时", "数据状态"]
        ]}
      />
      {state.loading && <EmptyState title="正在加载求助内容..." />}
      {state.error && state.items.length === 0 && <EmptyState title="求助内容加载失败" description={state.error} />}
      {!state.loading && !state.error && questions.length === 0 && (
        <EmptyState
          title="暂无求助内容"
          description="发布第一条求助后会出现在这里。"
          action={<button type="button" onClick={() => navigate("/question/create")}>发布求助</button>}
        />
      )}
      {!state.loading && questions.length > 0 && (
        <div className="question-stack">
          {questions.map((question) => (
            <QuestionCard question={question} key={question.id} />
          ))}
        </div>
      )}
      {state.offset < state.total && (
        <div className="dashboard-history-more">
          <span>{state.loadingMore ? "正在加载更多求助..." : state.error || "继续查看更多求助。"}</span>
          <button aria-label="加载更多求助" type="button" disabled={state.loadingMore} onClick={loadMoreQuestions}>
            {state.loadingMore ? "加载中" : "加载更多"}
          </button>
        </div>
      )}
      <section className="panel content-block">
        <BlockHeader icon={Users} title="最新回复线索" action="发布求助" onAction={() => navigate("/question/create")} />
        <div className="compact-list">
          {questions.length === 0 && <ListRow title="暂无回复线索" meta="有评论或互动后会显示在这里" />}
          {questions.slice(0, 4).map((question) => (
            <ListRow key={question.id} title={question.title} meta={`${question.answers} 个回答 · ${question.tags.slice(0, 3).join(" / ") || "未设置标签"}`} />
          ))}
        </div>
      </section>
    </>
  );
}

export function ResourcesPage() {
  const navigate = useNavigate();
  const [state, setState] = React.useState({ items: [], total: 0, offset: 0, loading: true, loadingMore: false, error: "" });

  const loadResources = React.useCallback((offset = 0, appending = false) => {
    let alive = true;
    setState((current) => ({
      ...current,
      items: appending ? current.items : [],
      total: appending ? current.total : 0,
      offset: appending ? current.offset : 0,
      loading: !appending,
      loadingMore: appending,
      error: ""
    }));
    bbsApi
      .links({ limit: RESOURCE_PAGE_SIZE, offset })
      .then((data) => {
        if (!alive) return;
        const pageItems = listItems(data);
        if (appending) {
          setState((current) => {
            const keys = new Set(current.items.map((item) => String(item?.id ?? item?.key ?? item?.url ?? item?.URL ?? "")));
            const items = [
              ...current.items,
              ...pageItems.filter((item) => {
                const key = String(item?.id ?? item?.key ?? item?.url ?? item?.URL ?? "");
                if (keys.has(key)) return false;
                keys.add(key);
                return true;
              })
            ];
            const nextOffset = current.offset + pageItems.length;
            return {
              ...current,
              items,
              total: pageItems.length > 0 ? Math.max(nextOffset, listTotal(data, pageItems)) : current.offset,
              offset: nextOffset,
              loadingMore: false,
              error: ""
            };
          });
          return;
        }
        setState({
          items: pageItems,
          total: Math.max(listTotal(data, pageItems), pageItems.length),
          offset: pageItems.length,
          loading: false,
          loadingMore: false,
          error: ""
        });
      })
      .catch((error) => {
        if (!alive) return;
        if (appending) {
          setState((current) => ({ ...current, loadingMore: false, error: error.message || "更多资源加载失败" }));
          return;
        }
        setState({ items: [], total: 0, offset: 0, loading: false, loadingMore: false, error: error.message || "资源加载失败" });
      });
    return () => {
      alive = false;
    };
  }, []);

  React.useEffect(loadResources, [loadResources]);

  function loadMoreResources() {
    if (state.loading || state.loadingMore || state.offset >= state.total) return;
    loadResources(state.offset, true);
  }

  const resources = state.items.map(linkToResource);

  return (
    <>
      <PageHero
        icon={FolderOpen}
        eyebrow="资源"
        title="把可复用的经验整理成资产"
        description="模板、清单、工具脚本和学习路线集中沉淀，方便团队在下一次项目里直接复用。"
        image={pageImages.资源}
        stats={[
          [state.loading ? "..." : String(state.total), "资源入口"],
          [state.error ? "失败" : "实时", "同步状态"],
          [String(resources.filter((item) => item.url).length), "可访问"]
        ]}
      />
      {state.loading && <EmptyState title="正在加载资源..." />}
      {state.error && state.items.length === 0 && <EmptyState title="资源加载失败" description={state.error} />}
      {!state.loading && !state.error && resources.length === 0 && <EmptyState title="暂无资源" description="在管理端维护友情链接后会展示在这里。" />}
      {!state.loading && resources.length > 0 && (
        <div className="resource-grid">
          {resources.map((resource) => (
            <ResourceCard resource={resource} key={resource.key} />
          ))}
        </div>
      )}
      {state.offset < state.total && (
        <div className="dashboard-history-more">
          <span>{state.loadingMore ? "正在加载更多资源..." : state.error || "继续查看更多资源。"}</span>
          <button aria-label="加载更多资源" type="button" disabled={state.loadingMore} onClick={loadMoreResources}>
            {state.loadingMore ? "加载中" : "加载更多"}
          </button>
        </div>
      )}
      <section className="panel content-block">
        <BlockHeader icon={Activity} title="资源活跃度" action="全部资源" onAction={() => navigate("/links")} />
        <div className="trend-bars">
          {resources.length === 0 && <ListRow title="暂无资源趋势" meta="资源上线后会显示活跃度" />}
          {resources.slice(0, 3).map((resource, index) => (
            <TrendBar key={resource.key} label={resource.title} value={Math.max(18, 76 - index * 14)} />
          ))}
        </div>
      </section>
    </>
  );
}

export function ShopPage({ auth }) {
  const token = auth?.accessToken || "";
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const {
    productId: linkedProductId,
    reviewOrderId: linkedReviewOrderId,
    couponCode: linkedCouponCode,
    category: linkedCategory,
    keyword: linkedKeyword
  } = parseShopDeepLink(searchParams);
  const [state, setState] = React.useState({ items: [], total: 0, offset: 0, loading: true, loadingMore: false, error: "" });
  const [filters, setFilters] = React.useState(() => ({ keyword: linkedKeyword, category: linkedCategory }));
  const [keywordDraft, setKeywordDraft] = React.useState(linkedKeyword);
  const [categoryOptions, setCategoryOptions] = React.useState([]);
  const [balance, setBalance] = React.useState(null);
  const [orders, setOrders] = React.useState([]);
  const [cart, setCart] = React.useState({ items: [], total: 0, loading: false, error: "", action: "" });
  const [favorites, setFavorites] = React.useState({ items: [], total: 0, offset: 0, ids: new Set(), loading: false, loadingMore: false, error: "", action: "" });
  const [coupons, setCoupons] = React.useState({ items: [], total: 0, offset: 0, loading: true, loadingMore: false, error: "", action: "" });
  const [myCoupons, setMyCoupons] = React.useState({ items: [], total: 0, offset: 0, loading: false, loadingMore: false, error: "" });
  const [addresses, setAddresses] = React.useState([]);
  const [addressPage, setAddressPage] = React.useState({ total: 0, offset: 0, loading: false, loadingMore: false, error: "" });
  const [fulfillment, setFulfillment] = React.useState(() => emptyFulfillment(auth?.user?.nickname || ""));
  const [selectedAddressId, setSelectedAddressId] = React.useState("");
  const [detailProduct, setDetailProduct] = React.useState(null);
  const [productReviews, setProductReviews] = React.useState({ items: [], total: 0, offset: 0, loading: false, loadingMore: false, error: "" });
  const [myProductReviews, setMyProductReviews] = React.useState({ items: [], total: 0, offset: 0, loading: false, loadingMore: false, error: "" });
  const [productReviewOrders, setProductReviewOrders] = React.useState({ items: [], total: 0, offset: 0, loading: false, loadingMore: false, error: "" });
  const [reviewForm, setReviewForm] = React.useState({ orderId: "", rating: 5, content: "", action: "", error: "" });
  const [reviewActionBusy, setReviewActionBusy] = React.useState(false);
  const [checkoutActionBusy, setCheckoutActionBusy] = React.useState(false);
  const [checkout, setCheckout] = React.useState({ product: null, items: [], mode: "", quantity: 1, couponCode: "", error: "" });
  const [notice, setNotice] = React.useState("");
  const [checkoutResultOrderId, setCheckoutResultOrderId] = React.useState("");
  const [addressAction, setAddressAction] = React.useState("");
  const [editingAddressId, setEditingAddressId] = React.useState("");
  const [busyProductId, setBusyProductId] = React.useState(null);
  const appliedLinkedCouponRef = React.useRef("");
  const checkoutSubmittingRef = React.useRef(0);
  const checkoutRequestIdRef = React.useRef(0);
  const reviewActionSubmittingRef = React.useRef(false);
  const shopSessionRef = React.useRef(0);
  const shopTokenRef = React.useRef(token);
  const productLoadRequestVersionRef = React.useRef(0);
  const productQueryRef = React.useRef({ keyword: filters.keyword, category: filters.category });
  const detailReviewSessionRef = React.useRef(0);
  const detailProductIdRef = React.useRef("");
  productQueryRef.current = { keyword: filters.keyword, category: filters.category };
  shopTokenRef.current = token;
  detailProductIdRef.current = String(detailProduct?.id || "");

  function isCurrentProductRequest(query, requestVersion) {
    const currentQuery = productQueryRef.current;
    return (
      requestVersion === productLoadRequestVersionRef.current &&
      query.keyword === currentQuery.keyword &&
      query.category === currentQuery.category
    );
  }

  function isCurrentShopSessionRequest(requestToken, session) {
    return session === shopSessionRef.current && requestToken === shopTokenRef.current;
  }

  function isCurrentDetailReviewRequest(productId, session) {
    return session === detailReviewSessionRef.current && String(productId || "") === detailProductIdRef.current;
  }

  React.useLayoutEffect(() => {
    productLoadRequestVersionRef.current += 1;
  }, [filters.category, filters.keyword]);

  React.useLayoutEffect(() => {
    shopSessionRef.current += 1;
  }, [token]);

  React.useEffect(() => {
    let alive = true;
    const query = { keyword: filters.keyword, category: filters.category };
    const requestVersion = ++productLoadRequestVersionRef.current;
    const isCurrentRequest = () => alive && isCurrentProductRequest(query, requestVersion);
    setState({ items: [], total: 0, offset: 0, loading: true, loadingMore: false, error: "" });
    bbsApi
      .mallProducts({ limit: SHOP_PRODUCT_PAGE_SIZE, offset: 0, ...query })
      .then((data) => {
        if (!isCurrentRequest()) return;
        const items = listItems(data);
        setState({ items, total: Math.max(listTotal(data, items), items.length), offset: items.length, loading: false, loadingMore: false, error: "" });
      })
      .catch((error) => {
        if (!isCurrentRequest()) return;
        setState({ items: [], total: 0, offset: 0, loading: false, loadingMore: false, error: error.message || "商品加载失败" });
      });
    return () => {
      alive = false;
    };
  }, [filters.category, filters.keyword]);

  React.useEffect(() => () => {
    productLoadRequestVersionRef.current += 1;
  }, []);

  React.useEffect(() => {
    let alive = true;
    loadAllListPages(bbsApi.mallCategories, { limit: SHOP_CATEGORY_PAGE_SIZE, offset: 0 })
      .then(({ items }) => {
        if (!alive) return;
        setCategoryOptions(mallCategoryOptions(items));
      })
      .catch(() => loadAllListPages(bbsApi.mallProducts, { limit: SHOP_CATEGORY_PAGE_SIZE, offset: 0 }))
      .then((data) => {
        if (!alive) return;
        if (data) {
          setCategoryOptions(mallCategoryOptions(data.items));
        }
      })
      .catch(() => {
        if (!alive) return;
        setCategoryOptions([]);
      });
    return () => {
      alive = false;
    };
  }, []);

  React.useEffect(() => {
    let alive = true;
    setCoupons((current) => ({ ...current, items: [], total: 0, offset: 0, loading: true, loadingMore: false, error: "" }));
    bbsApi
      .mallCoupons({ limit: SHOP_COUPON_PAGE_SIZE, offset: 0 })
      .then((data) => {
        if (!alive) return;
        setCoupons((current) => ({ ...current, ...couponPageState(data) }));
      })
      .catch((error) => {
        if (!alive) return;
        setCoupons((current) => ({ ...current, items: [], total: 0, offset: 0, loading: false, loadingMore: false, error: error.message || "优惠券加载失败" }));
      });
    return () => {
      alive = false;
    };
  }, []);

  React.useEffect(() => {
    checkoutSubmittingRef.current = 0;
    setCheckoutActionBusy(false);
    setBusyProductId(null);
    setCheckout({ product: null, items: [], mode: "", quantity: 1, couponCode: linkedCouponCode || "", error: "" });
    setCheckoutResultOrderId("");
    if (!token) {
      setBalance(null);
      setOrders([]);
      setCart({ items: [], total: 0, loading: false, error: "", action: "" });
      setFavorites({ items: [], total: 0, offset: 0, ids: new Set(), loading: false, loadingMore: false, error: "", action: "" });
      setMyCoupons({ items: [], total: 0, offset: 0, loading: false, loadingMore: false, error: "" });
      setAddresses([]);
      setAddressPage({ total: 0, offset: 0, loading: false, loadingMore: false, error: "" });
      setSelectedAddressId("");
      setEditingAddressId("");
      return;
    }
    let alive = true;
    const session = shopSessionRef.current;
    const isCurrentRequest = () => alive && isCurrentShopSessionRequest(token, session);
    setCart((current) => ({ ...current, loading: true, error: "" }));
    setFavorites({ items: [], total: 0, offset: 0, ids: new Set(), loading: true, loadingMore: false, error: "", action: "" });
    setMyCoupons({ items: [], total: 0, offset: 0, loading: true, loadingMore: false, error: "" });
    setAddressPage({ total: 0, offset: 0, loading: true, loadingMore: false, error: "" });
    Promise.allSettled([
      bbsApi.creditBalance(token),
      bbsApi.mallOrders({ limit: 5, offset: 0 }, token),
      bbsApi.mallAddresses({ limit: SHOP_ADDRESS_PAGE_SIZE, offset: 0 }, token),
      bbsApi.mallCart(token),
      bbsApi.mallProductFavorites({ limit: SHOP_FAVORITE_PAGE_SIZE, offset: 0 }, token),
      bbsApi.mallMyCoupons({ limit: SHOP_COUPON_PAGE_SIZE, offset: 0, status: COUPON_USAGE_STATUS_CLAIMED }, token)
    ]).then(([balanceResult, orderResult, addressResult, cartResult, favoriteResult, myCouponResult]) => {
        if (!isCurrentRequest()) return;
        setBalance(balanceResult.status === "fulfilled" ? balanceResult.value?.balance || null : null);
        setOrders(orderResult.status === "fulfilled" ? listItems(orderResult.value) : []);
        if (addressResult.status === "fulfilled") {
          applyAddressList(listItems(addressResult.value), addressResult.value);
        } else {
          setAddresses([]);
          setAddressPage({ total: 0, offset: 0, loading: false, loadingMore: false, error: addressResult.reason?.message || "收货地址加载失败" });
        }
        if (cartResult.status === "fulfilled") {
          applyCartData(cartResult.value);
        } else {
          setCart({ items: [], total: 0, loading: false, error: cartResult.reason?.message || "购物车加载失败", action: "" });
        }
        if (favoriteResult.status === "fulfilled") {
          applyFavoriteData(favoriteResult.value);
        } else {
          setFavorites({ items: [], total: 0, offset: 0, ids: new Set(), loading: false, loadingMore: false, error: favoriteResult.reason?.message || "收藏商品加载失败", action: "" });
        }
        if (myCouponResult.status === "fulfilled") {
          setMyCoupons(couponPageState(myCouponResult.value));
        } else {
          setMyCoupons({ items: [], total: 0, offset: 0, loading: false, loadingMore: false, error: myCouponResult.reason?.message || "我的优惠券加载失败" });
        }
      });
    return () => {
      alive = false;
    };
  }, [token]);

  React.useEffect(() => {
    setFulfillment((current) => ({ ...current, receiver: current.receiver || auth?.user?.nickname || "" }));
  }, [auth?.user?.nickname]);

  React.useLayoutEffect(() => {
    detailReviewSessionRef.current += 1;
  }, [detailProduct?.id, token]);

  React.useEffect(() => {
    const reviewSession = detailReviewSessionRef.current;
    const productId = detailProduct?.id;
    if (!productId) {
      setProductReviews({ items: [], total: 0, offset: 0, loading: false, loadingMore: false, error: "" });
      setMyProductReviews({ items: [], total: 0, offset: 0, loading: false, loadingMore: false, error: "" });
      setProductReviewOrders({ items: [], total: 0, offset: 0, loading: false, loadingMore: false, error: "" });
      setReviewForm({ orderId: "", rating: 5, content: "", action: "", error: "" });
      return;
    }
    let alive = true;
    const isCurrentRequest = () => alive && isCurrentDetailReviewRequest(productId, reviewSession);
    setProductReviews({ items: [], total: 0, offset: 0, loading: true, loadingMore: false, error: "" });
    bbsApi
      .mallProductReviews(productId, { limit: SHOP_PRODUCT_REVIEW_PAGE_SIZE, offset: 0 })
      .then((data) => {
        if (!isCurrentRequest()) return;
        setProductReviews(productReviewPageState(data));
      })
      .catch((error) => {
        if (!isCurrentRequest()) return;
        setProductReviews({ items: [], total: 0, offset: 0, loading: false, loadingMore: false, error: error.message || "评价加载失败" });
      });
    if (token) {
      setProductReviewOrders({ items: [], total: 0, offset: 0, loading: true, loadingMore: false, error: "" });
      setMyProductReviews({ items: [], total: 0, offset: 0, loading: true, loadingMore: false, error: "" });
      bbsApi
        .mallReviewableOrders(productId, { limit: SHOP_REVIEWABLE_ORDER_PAGE_SIZE, offset: 0 }, token)
        .then((data) => {
          if (!isCurrentRequest()) return;
          setProductReviewOrders(productReviewOrderPageState(data));
        })
        .catch((error) => {
          if (!isCurrentRequest()) return;
          setProductReviewOrders({ items: [], total: 0, offset: 0, loading: false, loadingMore: false, error: error.message || "可评价订单加载失败" });
        });
      bbsApi
        .mallReviews({ limit: SHOP_PRODUCT_REVIEW_PAGE_SIZE, offset: 0, product_id: productId }, token)
        .then((data) => {
          if (!isCurrentRequest()) return;
          setMyProductReviews(productReviewPageState(data));
        })
        .catch((error) => {
          if (!isCurrentRequest()) return;
          setMyProductReviews({ items: [], total: 0, offset: 0, loading: false, loadingMore: false, error: error.message || "我的评价加载失败" });
        });
    } else {
      setProductReviewOrders({ items: [], total: 0, offset: 0, loading: false, loadingMore: false, error: "" });
      setMyProductReviews({ items: [], total: 0, offset: 0, loading: false, loadingMore: false, error: "" });
    }
    return () => {
      alive = false;
    };
  }, [detailProduct?.id, token]);

  React.useEffect(() => {
    if (!linkedProductId) return;
    if (String(detailProduct?.id || "") === String(linkedProductId)) return;
    let alive = true;
    setNotice("");
    bbsApi
      .mallProduct(linkedProductId)
      .then((data) => {
        if (!alive) return;
        if (data?.product) {
          setDetailProduct(mallProductToCard(data.product, 0));
        } else {
          setDetailProduct(null);
          setNotice("商品详情不存在或已下架。");
        }
      })
      .catch((error) => {
        if (!alive) return;
        setDetailProduct(null);
        setNotice(error.message || "商品详情加载失败。");
      });
    return () => {
      alive = false;
    };
  }, [linkedProductId]);

  React.useEffect(() => {
    if (!linkedCouponCode) {
      appliedLinkedCouponRef.current = "";
      return;
    }
    if (appliedLinkedCouponRef.current === linkedCouponCode) return;
    appliedLinkedCouponRef.current = linkedCouponCode;
    setCheckout((current) => ({ ...current, couponCode: linkedCouponCode, error: "" }));
    setNotice(`优惠码 ${linkedCouponCode} 已带入，结算时会自动尝试抵扣。`);
  }, [linkedCouponCode]);

  React.useEffect(() => {
    setFilters((current) =>
      current.keyword === linkedKeyword && current.category === linkedCategory
        ? current
        : { keyword: linkedKeyword, category: linkedCategory }
    );
    setKeywordDraft((current) => (current === linkedKeyword ? current : linkedKeyword));
  }, [linkedCategory, linkedKeyword]);

  const favoriteIds = favorites.ids || new Set();
  const products = sortProductsForStorefront(
    state.items.map((item, index) => {
      const product = mallProductToCard(item, index);
      return { ...product, isFavorite: favoriteIds.has(String(product.id)) };
    })
  );
  const favoriteProducts = favorites.items.map(productFavoriteToCard);
  const totalStock = state.items.reduce((sum, item) => sum + toNumber(item.stock), 0);
  const activeFilters = Boolean(filters.keyword || filters.category);
  const cartItems = Array.isArray(cart.items) ? cart.items : [];
  const cartTotalQuantity = cartItems.reduce((sum, item) => sum + cartItemQuantity(item), 0);
  const cartTotalCredits = cartItems.reduce((sum, item) => sum + cartItemSubtotal(item), 0);
  const checkoutLines = checkoutCartLines(checkout);
  const checkoutCost = checkoutLines.reduce((sum, line) => sum + toNumber(line.product?.priceCredits) * toNumber(line.quantity), 0);
  const checkoutCouponCode = String(checkout.couponCode || "").trim().toUpperCase();
  const myClaimedCoupons = myCoupons.items.filter(couponUsageSelectable);
  const claimedCouponIds = new Set(myClaimedCoupons.map(couponIdOf).filter(Boolean).map(String));
  const claimedCouponCodes = new Set(myClaimedCoupons.map(couponCodeOf).filter(Boolean));
  const selectedCoupon = myClaimedCoupons.find((item) => couponCodeOf(item) === checkoutCouponCode);
  const selectedCouponAvailable = selectedCoupon ? mallCouponIsAvailable(selectedCoupon) : false;
  const selectedCouponUsable = selectedCoupon ? couponUsableForTotal(selectedCoupon, checkoutCost) : false;
  const checkoutDiscount = selectedCouponUsable ? Math.min(couponDiscountOf(selectedCoupon), checkoutCost) : 0;
  const checkoutPayableCost = Math.max(0, checkoutCost - checkoutDiscount);
  const checkoutCouponState = mallCouponCheckoutState({
    couponCode: checkoutCouponCode,
    selectedCoupon,
    selectedCouponAvailable,
    selectedCouponUsable
  });
  const checkoutCouponMessage = mallCouponCheckoutMessage({
    couponState: checkoutCouponState,
    couponCode: checkoutCouponCode,
    couponName: couponNameOf(selectedCoupon),
    discountCredits: checkoutDiscount,
    minOrderCredits: couponMinOrderOf(selectedCoupon)
  });
  const canAttemptCouponCheckout = checkoutCouponState.canSubmit;
  const hasUnverifiedCouponCode = checkoutCouponState.status === MALL_COUPON_CHECKOUT_STATUS.UNVERIFIED;
  const couponGuideProducts = selectedCouponAvailable ? couponRecommendedProducts(products, selectedCoupon).slice(0, 4) : [];
  const couponGuideVisible = Boolean(checkoutCouponCode && !checkout.mode);
  const balanceLoaded = Boolean(balance);
  const balanceTotal = balanceLoaded ? toNumber(balance?.total) : 0;
  const checkoutBalanceShortfall = balanceLoaded ? Math.max(0, checkoutPayableCost - balanceTotal) : 0;
  const checkoutBalanceBlocked = shouldBlockMallCheckoutForBalance({
    balanceShortfall: checkoutBalanceShortfall,
    couponState: checkoutCouponState
  });
  const checkoutRemaining = balanceLoaded ? Math.max(0, balanceTotal - checkoutPayableCost) : 0;
  const checkoutHasStockIssue = checkoutLines.some((line) => toNumber(line.quantity) <= 0 || toNumber(line.quantity) > toNumber(line.product?.stock));
  const checkoutRequiresShipping = checkoutLines.some((line) => productRequiresShipping(line.product));
  const checkoutFulfillmentText = checkoutRequiresShipping ? "" : checkoutDigitalFulfillmentText(checkoutLines);
  const checkoutBusy = checkoutActionBusy || busyProductId === (checkout.mode === "cart" ? "cart" : checkoutLines[0]?.product?.id);
  const pendingCheckoutOrderIds = checkoutAttemptOrderIds({ userId: auth?.user?.id });
  const resumableCheckoutOrder = orders.find(
    (order) => pendingCheckoutOrderIds.includes(String(order?.id || "")) && mallOrderCanPay(order)
  );
  const reviewableOrders = detailProduct ? productReviewableOrders(productReviewOrders.items, detailProduct.id) : [];
  const selectedReviewOrderId = reviewForm.orderId || reviewOrderIdIn(reviewableOrders, linkedReviewOrderId) || String(reviewableOrders[0]?.id || "");
  const showMyProductReviews = token && (myProductReviews.loading || myProductReviews.error || myProductReviews.items.length > 0);

  const goOrders = React.useCallback(
    (orderId) => {
      if (!token) {
        navigate("/user/signin");
        return;
      }
      const query = orderId ? `?order_id=${encodeURIComponent(orderId)}` : "";
      navigate(`/dashboard/orders${query}`);
    },
    [navigate, token]
  );
  const checkoutResultAction =
    checkoutResultOrderId && checkoutNoticeOpensOrder(notice) ? (
      <button type="button" onClick={() => goOrders(checkoutResultOrderId)}>
        <Activity size={16} aria-hidden="true" />
        查看订单
      </button>
    ) : null;

  async function refreshWallet() {
    const requestToken = token;
    const session = shopSessionRef.current;
    if (!requestToken || !isCurrentShopSessionRequest(requestToken, session)) return;
    const [balanceData, orderData] = await Promise.all([bbsApi.creditBalance(requestToken), bbsApi.mallOrders({ limit: 5, offset: 0 }, requestToken)]);
    if (!isCurrentShopSessionRequest(requestToken, session)) return;
    setBalance(balanceData?.balance || null);
    setOrders(listItems(orderData));
  }

  async function reloadProducts() {
    const query = { ...productQueryRef.current };
    const requestVersion = ++productLoadRequestVersionRef.current;
    const isCurrentRequest = () => isCurrentProductRequest(query, requestVersion);
    setState((current) => ({ ...current, loading: true, loadingMore: false, error: "" }));
    try {
      const data = await bbsApi.mallProducts({ limit: SHOP_PRODUCT_PAGE_SIZE, offset: 0, ...query });
      if (!isCurrentRequest()) return [];
      const items = listItems(data);
      setState({ items, total: Math.max(listTotal(data, items), items.length), offset: items.length, loading: false, loadingMore: false, error: "" });
      return items;
    } catch (error) {
      if (!isCurrentRequest()) return [];
      setState((current) => ({ ...current, loading: false, error: error.message || "商品加载失败" }));
      return [];
    }
  }

  async function loadMoreProducts() {
    if (state.loading || state.loadingMore || state.offset >= state.total) return;
    const offset = state.offset;
    const query = { keyword: filters.keyword, category: filters.category };
    const requestVersion = ++productLoadRequestVersionRef.current;
    const isCurrentRequest = () => isCurrentProductRequest(query, requestVersion);
    setState((current) => ({ ...current, loadingMore: true, error: "" }));
    try {
      const data = await bbsApi.mallProducts({ limit: SHOP_PRODUCT_PAGE_SIZE, offset, ...query });
      if (!isCurrentRequest()) return;
      const pageItems = listItems(data);
      setState((current) => {
        const knownIDs = new Set(current.items.map((item) => String(item?.id || "")).filter(Boolean));
        const items = [...current.items, ...pageItems.filter((item) => {
          const id = String(item?.id || "");
          if (!id || knownIDs.has(id)) return false;
          knownIDs.add(id);
          return true;
        })];
        const total = Math.max(listTotal(data, pageItems), items.length);
        return {
          ...current,
          items,
          total,
          offset: pageItems.length > 0 ? offset + pageItems.length : total,
          loadingMore: false,
          error: ""
        };
      });
    } catch (error) {
      if (!isCurrentRequest()) return;
      setState((current) => ({ ...current, loadingMore: false, error: error.message || "更多商品加载失败" }));
    }
  }

  function applyCartData(data) {
    const items = listItems(data);
    setCart((current) => ({
      ...current,
      items,
      total: listTotal(data, items),
      loading: false,
      error: "",
      action: ""
    }));
  }

  function applyFavoriteData(data) {
    const nextState = favoritePageState(data);
    setFavorites((current) => ({
      ...current,
      ...nextState,
      action: ""
    }));
  }

  async function reloadCart() {
    const requestToken = token;
    const session = shopSessionRef.current;
    if (!requestToken || !isCurrentShopSessionRequest(requestToken, session)) return [];
    setCart((current) => ({ ...current, loading: true, error: "" }));
    try {
      const data = await bbsApi.mallCart(requestToken);
      if (!isCurrentShopSessionRequest(requestToken, session)) return [];
      const items = listItems(data);
      setCart({ items, total: listTotal(data, items), loading: false, error: "", action: "" });
      return items;
    } catch (error) {
      if (!isCurrentShopSessionRequest(requestToken, session)) return [];
      setCart((current) => ({ ...current, loading: false, error: error.message || "购物车加载失败", action: "" }));
      return [];
    }
  }

  async function reloadFavorites() {
    if (!token) return [];
    setFavorites((current) => ({ ...current, loading: true, loadingMore: false, error: "" }));
    try {
      const data = await bbsApi.mallProductFavorites({ limit: SHOP_FAVORITE_PAGE_SIZE, offset: 0 }, token);
      const items = listItems(data);
      applyFavoriteData(data);
      return items;
    } catch (error) {
      setFavorites((current) => ({ ...current, loading: false, error: error.message || "收藏商品加载失败", action: "" }));
      return [];
    }
  }

  async function loadMoreFavorites() {
    if (!token || favorites.loading || favorites.loadingMore || favorites.offset >= favorites.total) return;
    const offset = favorites.offset;
    setFavorites((current) => ({ ...current, loadingMore: true, error: "" }));
    try {
      const data = await bbsApi.mallProductFavorites({ limit: SHOP_FAVORITE_PAGE_SIZE, offset }, token);
      const pageItems = listItems(data);
      setFavorites((current) => {
        const items = appendUniqueFavoriteItems(current.items, pageItems);
        const total = Math.max(listTotal(data, pageItems), items.length);
        return {
          ...current,
          items,
          total,
          offset: pageItems.length > 0 ? offset + pageItems.length : total,
          ids: favoriteProductIDSet(items),
          loadingMore: false,
          error: ""
        };
      });
    } catch (error) {
      setFavorites((current) => ({ ...current, loadingMore: false, error: error.message || "更多收藏商品加载失败" }));
    }
  }

  async function addToCart(product) {
    if (!token) {
      setNotice("请先登录后再加入购物车。");
      return;
    }
    if (checkout.mode === "cart" || checkoutSubmittingRef.current) return;
    const productId = product?.id;
    if (!productId) return;
    const existing = cartItems.find((item) => String(cartProductOf(item)?.id) === String(productId));
    const nextQuantity = Math.min(toNumber(product.stock), cartItemQuantity(existing) + 1 || 1);
    if (nextQuantity <= 0) {
      setNotice("当前商品库存不足。");
      return;
    }
    setCart((current) => ({ ...current, action: `add-${productId}`, error: "" }));
    setNotice("");
    try {
      const data = await bbsApi.setMallCartItem(productId, { quantity: nextQuantity }, token);
      applyCartData(data);
      setNotice("商品已加入购物车。");
    } catch (error) {
      setCart((current) => ({ ...current, action: "", error: friendlyMallCheckoutError(error) }));
    }
  }

  async function toggleProductFavorite(product) {
    if (!token) {
      setNotice("请先登录后再收藏商品。");
      return;
    }
    const productId = product?.id;
    if (!productId) return;
    const wasFavorited = favoriteIds.has(String(productId));
    setFavorites((current) => ({ ...current, action: `fav-${productId}`, error: "" }));
    setNotice("");
    try {
      if (wasFavorited) {
        await bbsApi.unfavoriteMallProduct(productId, token);
      } else {
        await bbsApi.favoriteMallProduct(productId, token);
      }
      await reloadFavorites();
      setNotice(wasFavorited ? "已取消收藏。" : "商品已收藏。");
    } catch (error) {
      setFavorites((current) => ({ ...current, action: "", error: error.message || "收藏操作失败" }));
    }
  }

  async function refreshCheckoutProduct(productId) {
    if (!productId) return null;
    try {
      const data = await bbsApi.mallProduct(productId);
      const product = data?.product ? mallProductToCard(data.product, 0) : null;
      if (!product) return null;
      setDetailProduct((current) => (String(current?.id || "") === String(productId) ? product : current));
      setCheckout((current) => {
        if (current.mode !== "single" || String(current.product?.id || "") !== String(productId)) {
          return current;
        }
        return {
          ...current,
          product,
          quantity: Math.max(1, Math.min(toNumber(product.stock), toNumber(current.quantity) || 1))
        };
      });
      return product;
    } catch {
      return null;
    }
  }

  async function syncCheckoutAfterMallError(error) {
    const requestToken = token;
    const session = shopSessionRef.current;
    const isCurrentRequest = () => isCurrentShopSessionRequest(requestToken, session);
    if (!isCurrentRequest()) return;
    const jobs = [];
    if (shouldRefreshMallInventoryAfterError(error)) {
      jobs.push(reloadProducts());
      if (checkout.mode === "cart") {
        jobs.push(
          reloadCart().then((items) => {
            if (!isCurrentRequest()) return items;
            setCheckout((current) => (current.mode === "cart" ? { ...current, items } : current));
            return items;
          })
        );
      } else {
        jobs.push(refreshCheckoutProduct(checkoutLines[0]?.product?.id));
      }
    }
    if (shouldRefreshMallCouponsAfterError(error)) {
      jobs.push(refreshCoupons());
      jobs.push(refreshMyCoupons());
    }
    if (jobs.length > 0) {
      await Promise.allSettled(jobs);
    }
  }

  async function updateCartQuantity(item, quantity) {
    const product = cartProductOf(item);
    const productId = product?.id;
    if (!token || !productId || checkout.mode === "cart" || checkoutSubmittingRef.current) return;
    const nextQuantity = Math.max(1, Math.min(toNumber(product.stock), toNumber(quantity) || 1));
    setCart((current) => ({ ...current, action: `qty-${productId}`, error: "" }));
    try {
      const data = await bbsApi.setMallCartItem(productId, { quantity: nextQuantity }, token);
      applyCartData(data);
    } catch (error) {
      setCart((current) => ({ ...current, action: "", error: friendlyMallCheckoutError(error) }));
    }
  }

  async function removeCartItem(item) {
    const productId = cartProductOf(item)?.id;
    if (!token || !productId || checkout.mode === "cart" || checkoutSubmittingRef.current) return;
    setCart((current) => ({ ...current, action: `remove-${productId}`, error: "" }));
    try {
      const data = await bbsApi.removeMallCartItem(productId, token);
      applyCartData(data);
    } catch (error) {
      setCart((current) => ({ ...current, action: "", error: error.message || "移除购物车失败" }));
    }
  }

  async function clearCart() {
    if (!token || cartItems.length === 0 || checkout.mode === "cart" || checkoutSubmittingRef.current) return;
    setCart((current) => ({ ...current, action: "clear", error: "" }));
    try {
      const data = await bbsApi.clearMallCart(token);
      applyCartData(data);
    } catch (error) {
      setCart((current) => ({ ...current, action: "", error: error.message || "清空购物车失败" }));
    }
  }

  function applyAddressList(items, data) {
    setAddresses(items);
    setAddressPage({
      total: Math.max(listTotal(data, items), items.length),
      offset: items.length,
      loading: false,
      loadingMore: false,
      error: ""
    });
    if (items.length === 0) {
      setSelectedAddressId("");
      setEditingAddressId("");
      return;
    }
    const selected = items.find((item) => String(item.id) === String(selectedAddressId));
    const fallback = selected || items.find((item) => item.is_default || item.isDefault) || items[0];
    setSelectedAddressId(String(fallback.id));
    if (editingAddressId && !items.some((item) => String(item.id) === editingAddressId)) {
      setEditingAddressId("");
    }
    setFulfillment(addressToFulfillment(fallback));
  }

  async function reloadAddresses() {
    if (!token) return [];
    const data = await bbsApi.mallAddresses({ limit: SHOP_ADDRESS_PAGE_SIZE, offset: 0 }, token);
    const items = listItems(data);
    applyAddressList(items, data);
    return items;
  }

  async function loadMoreAddresses() {
    if (!token || addressPage.loading || addressPage.loadingMore || addressPage.offset >= addressPage.total) return;
    const offset = addressPage.offset;
    setAddressPage((current) => ({ ...current, loadingMore: true, error: "" }));
    try {
      const data = await bbsApi.mallAddresses({ limit: SHOP_ADDRESS_PAGE_SIZE, offset }, token);
      const pageItems = listItems(data);
      setAddresses((current) => appendUniqueAddressItems(current, pageItems));
      setAddressPage((current) => {
        const nextOffset = current.offset + pageItems.length;
        return {
          ...current,
          total: pageItems.length > 0 ? Math.max(nextOffset, listTotal(data, pageItems)) : nextOffset,
          offset: nextOffset,
          loadingMore: false,
          error: ""
        };
      });
    } catch (error) {
      setAddressPage((current) => ({ ...current, loadingMore: false, error: error.message || "更多收货地址加载失败" }));
    }
  }

  async function refreshCoupons() {
    const data = await bbsApi.mallCoupons({ limit: SHOP_COUPON_PAGE_SIZE, offset: 0 });
    const nextState = couponPageState(data);
    setCoupons((current) => ({ ...current, ...nextState }));
    return nextState.items;
  }

  async function refreshMyCoupons() {
    const requestToken = token;
    const session = shopSessionRef.current;
    if (!requestToken) {
      if (!isCurrentShopSessionRequest(requestToken, session)) return [];
      setMyCoupons({ items: [], total: 0, offset: 0, loading: false, loadingMore: false, error: "" });
      return [];
    }
    if (!isCurrentShopSessionRequest(requestToken, session)) return [];
    const data = await bbsApi.mallMyCoupons({ limit: SHOP_COUPON_PAGE_SIZE, offset: 0, status: COUPON_USAGE_STATUS_CLAIMED }, requestToken);
    if (!isCurrentShopSessionRequest(requestToken, session)) return [];
    const nextState = couponPageState(data);
    setMyCoupons(nextState);
    return nextState.items;
  }

  async function loadMoreCoupons() {
    if (coupons.loading || coupons.loadingMore || coupons.offset >= coupons.total) return;
    const offset = coupons.offset;
    setCoupons((current) => ({ ...current, loadingMore: true, error: "" }));
    try {
      const data = await bbsApi.mallCoupons({ limit: SHOP_COUPON_PAGE_SIZE, offset });
      const pageItems = listItems(data);
      setCoupons((current) => {
        const items = appendUniqueCouponItems(current.items, pageItems);
        const total = Math.max(listTotal(data, pageItems), items.length);
        return {
          ...current,
          items,
          total,
          offset: pageItems.length > 0 ? offset + pageItems.length : total,
          loadingMore: false,
          error: ""
        };
      });
    } catch (error) {
      setCoupons((current) => ({ ...current, loadingMore: false, error: error.message || "更多优惠券加载失败" }));
    }
  }

  async function loadMoreMyCoupons() {
    if (!token || myCoupons.loading || myCoupons.loadingMore || myCoupons.offset >= myCoupons.total) return;
    const offset = myCoupons.offset;
    setMyCoupons((current) => ({ ...current, loadingMore: true, error: "" }));
    try {
      const data = await bbsApi.mallMyCoupons({ limit: SHOP_COUPON_PAGE_SIZE, offset, status: COUPON_USAGE_STATUS_CLAIMED }, token);
      const pageItems = listItems(data);
      setMyCoupons((current) => {
        const items = appendUniqueCouponItems(current.items, pageItems);
        const total = Math.max(listTotal(data, pageItems), items.length);
        return {
          ...current,
          items,
          total,
          offset: pageItems.length > 0 ? offset + pageItems.length : total,
          loadingMore: false,
          error: ""
        };
      });
    } catch (error) {
      setMyCoupons((current) => ({ ...current, loadingMore: false, error: error.message || "更多我的优惠券加载失败" }));
    }
  }

  async function claimCoupon(coupon) {
    const couponId = couponIdOf(coupon);
    if (!token) {
      setNotice("请先登录后再领取优惠券。");
      return;
    }
    if (!couponId) return;
    const action = `claim-${couponId}`;
    setCoupons((current) => ({ ...current, action, error: "" }));
    setNotice("");
    try {
      const data = await bbsApi.claimMallCoupon(couponId, token);
      const [couponResult, myCouponResult] = await Promise.allSettled([refreshCoupons(), refreshMyCoupons()]);
      if (couponResult.status === "rejected") {
        setCoupons((current) => ({ ...current, loading: false, error: couponResult.reason?.message || "优惠券列表刷新失败" }));
      }
      if (myCouponResult.status === "rejected") {
        setMyCoupons((current) => ({ ...current, loading: false, error: myCouponResult.reason?.message || "我的优惠券刷新失败" }));
      }
      setNotice(data?.duplicate || data?.already_claimed || data?.alreadyClaimed ? "这张优惠券已经在你的券包里。" : "优惠券已领取，结算时可从“我的优惠券”选择。");
    } catch (error) {
      setCoupons((current) => ({ ...current, error: error.message || "优惠券领取失败" }));
    } finally {
      setCoupons((current) => ({ ...current, action: "" }));
    }
  }

  function useAddress(address) {
    setSelectedAddressId(String(address.id));
    setFulfillment(addressToFulfillment(address));
    setEditingAddressId("");
    setNotice("");
  }

  function editAddress(address) {
    setSelectedAddressId(String(address.id));
    setFulfillment(addressToFulfillment(address));
    setEditingAddressId(String(address.id));
    setNotice("");
  }

  function cancelAddressEdit() {
    const selected = addresses.find((address) => String(address.id) === String(editingAddressId || selectedAddressId));
    setEditingAddressId("");
    if (selected) {
      setSelectedAddressId(String(selected.id));
      setFulfillment(addressToFulfillment(selected));
    }
  }

  function updateShopSearchParams(changes, options) {
    const params = new URLSearchParams(searchParams);
    Object.entries(changes).forEach(([key, value]) => {
      if (value === undefined || value === null || value === "") {
        params.delete(key);
      } else {
        params.set(key, String(value));
      }
    });
    setSearchParams(params, options);
  }

  async function saveAddress() {
    if (!token) {
      setNotice("请先登录后再保存收货地址。");
      return;
    }
    const receiver = fulfillment.receiver.trim();
    const phone = fulfillment.phone.trim();
    const detail = fulfillment.detail.trim();
    if (!receiver || !phone || !detail) {
      setNotice("请先补全收件人、联系电话和详细地址。");
      return;
    }
    setAddressAction("save");
    setNotice("");
    try {
      const currentAddress = addresses.find((address) => String(address.id) === String(editingAddressId));
      const payload = {
        receiver,
        phone,
        detail,
        province: fulfillment.province.trim(),
        city: fulfillment.city.trim(),
        district: fulfillment.district.trim(),
        postal_code: fulfillment.postalCode.trim(),
        is_default: editingAddressId ? Boolean(currentAddress?.is_default || currentAddress?.isDefault) : addresses.length === 0
      };
      const data = editingAddressId
        ? await bbsApi.updateMallAddress(editingAddressId, payload, token)
        : await bbsApi.createMallAddress(payload, token);
      await reloadAddresses();
      if (data?.address) {
        useAddress(data.address);
      }
      setEditingAddressId("");
      setNotice(editingAddressId ? "收货地址已更新。" : "收货地址已保存。");
    } catch (error) {
      setNotice(error.message || (editingAddressId ? "收货地址更新失败。" : "收货地址保存失败。"));
    } finally {
      setAddressAction("");
    }
  }

  async function setDefaultAddress(address) {
    if (!token || !address?.id) return;
    setAddressAction(`default-${address.id}`);
    setNotice("");
    try {
      await bbsApi.setDefaultMallAddress(address.id, token);
      await reloadAddresses();
      setNotice("默认收货地址已更新。");
    } catch (error) {
      setNotice(error.message || "默认地址设置失败。");
    } finally {
      setAddressAction("");
    }
  }

  async function deleteAddress(address) {
    if (!token || !address?.id) return;
    setAddressAction(`delete-${address.id}`);
    setNotice("");
    try {
      await bbsApi.deleteMallAddress(address.id, token);
      const items = await reloadAddresses();
      if (items.length === 0) {
        setSelectedAddressId("");
      }
      setNotice("收货地址已删除。");
    } catch (error) {
      setNotice(error.message || "收货地址删除失败。");
    } finally {
      setAddressAction("");
    }
  }

  function openCheckout(product) {
    if (!token) {
      setNotice("请先登录后再兑换商品。");
      return;
    }
    if (checkoutSubmittingRef.current) return;
    setCheckout((current) => ({ product, items: [], mode: "single", quantity: 1, couponCode: current.couponCode || "", error: "" }));
    closeProductDetail();
    setNotice("");
  }

  function openCouponCheckout(product) {
    if (!token) {
      setNotice("请先登录后再使用优惠券。");
      return;
    }
    if (checkoutSubmittingRef.current) return;
    const quantity = couponSuggestedQuantity(product, selectedCoupon);
    setCheckout({ product, items: [], mode: "single", quantity, couponCode: checkoutCouponCode, error: "" });
    closeProductDetail();
    setNotice(`已选择 ${product.title}${quantity > 1 ? ` x${quantity}` : ""}，优惠码 ${checkoutCouponCode} 将在结算时抵扣。`);
  }

  function openProductDetail(product) {
    setDetailProduct(product);
    if (product?.id) {
      updateShopSearchParams({ product_id: product.id, review_order_id: "" });
    }
  }

  function closeProductDetail() {
    setDetailProduct(null);
    if (linkedProductId || linkedReviewOrderId) {
      updateShopSearchParams({ product_id: "", review_order_id: "" }, { replace: true });
    }
  }

  function openCartCheckout() {
    if (!token) {
      setNotice("请先登录后再结算购物车。");
      return;
    }
    if (cart.loading || cart.action) {
      setNotice("购物车正在更新，请稍候再结算。");
      return;
    }
    if (cartItems.length === 0 || checkout.mode === "cart" || checkoutSubmittingRef.current) {
      setNotice("购物车暂无商品。");
      return;
    }
    setCheckout((current) => ({ product: null, items: cartItems, mode: "cart", quantity: 1, couponCode: current.couponCode || "", error: "" }));
    closeProductDetail();
    setNotice("");
  }

  function submitFilters(event) {
    event.preventDefault();
    const keyword = keywordDraft.trim();
    setFilters((current) => ({ ...current, keyword }));
    updateShopSearchParams({ keyword }, { replace: true });
  }

  function changeCategory(category) {
    setFilters((current) => ({ ...current, category }));
    updateShopSearchParams({ category }, { replace: true });
  }

  function clearFilters() {
    setKeywordDraft("");
    setFilters({ keyword: "", category: "" });
    updateShopSearchParams({ keyword: "", category: "" }, { replace: true });
  }

  async function loadMoreProductReviews() {
    if (!detailProduct?.id || productReviews.loading || productReviews.loadingMore || productReviews.offset >= productReviews.total) return;
    const productId = detailProduct.id;
    const reviewSession = detailReviewSessionRef.current;
    const isCurrentRequest = () => isCurrentDetailReviewRequest(productId, reviewSession);
    const offset = productReviews.offset;
    setProductReviews((current) => ({ ...current, loadingMore: true, error: "" }));
    try {
      const data = await bbsApi.mallProductReviews(productId, { limit: SHOP_PRODUCT_REVIEW_PAGE_SIZE, offset });
      if (!isCurrentRequest()) return;
      const pageItems = listItems(data);
      setProductReviews((current) => {
        const items = appendUniqueProductReviews(current.items, pageItems);
        const total = Math.max(listTotal(data, pageItems), items.length);
        return {
          ...current,
          items,
          total,
          offset: pageItems.length > 0 ? offset + pageItems.length : total,
          loadingMore: false,
          error: ""
        };
      });
    } catch (error) {
      if (!isCurrentRequest()) return;
      setProductReviews((current) => ({ ...current, loadingMore: false, error: error.message || "更多商品评价加载失败" }));
    }
  }

  async function loadMoreMyProductReviews() {
    if (!token || !detailProduct?.id || myProductReviews.loading || myProductReviews.loadingMore || myProductReviews.offset >= myProductReviews.total) return;
    const productId = detailProduct.id;
    const reviewSession = detailReviewSessionRef.current;
    const isCurrentRequest = () => isCurrentDetailReviewRequest(productId, reviewSession);
    const offset = myProductReviews.offset;
    setMyProductReviews((current) => ({ ...current, loadingMore: true, error: "" }));
    try {
      const data = await bbsApi.mallReviews({ limit: SHOP_PRODUCT_REVIEW_PAGE_SIZE, offset, product_id: productId }, token);
      if (!isCurrentRequest()) return;
      const pageItems = listItems(data);
      setMyProductReviews((current) => {
        const items = appendUniqueProductReviews(current.items, pageItems);
        const total = Math.max(listTotal(data, pageItems), items.length);
        return {
          ...current,
          items,
          total,
          offset: pageItems.length > 0 ? offset + pageItems.length : total,
          loadingMore: false,
          error: ""
        };
      });
    } catch (error) {
      if (!isCurrentRequest()) return;
      setMyProductReviews((current) => ({ ...current, loadingMore: false, error: error.message || "更多我的评价加载失败" }));
    }
  }

  async function loadMoreProductReviewOrders() {
    if (!token || !detailProduct?.id || productReviewOrders.loading || productReviewOrders.loadingMore || productReviewOrders.offset >= productReviewOrders.total) return;
    const productId = detailProduct.id;
    const reviewSession = detailReviewSessionRef.current;
    const isCurrentRequest = () => isCurrentDetailReviewRequest(productId, reviewSession);
    const offset = productReviewOrders.offset;
    setProductReviewOrders((current) => ({ ...current, loadingMore: true, error: "" }));
    try {
      const data = await bbsApi.mallReviewableOrders(productId, { limit: SHOP_REVIEWABLE_ORDER_PAGE_SIZE, offset }, token);
      if (!isCurrentRequest()) return;
      const pageItems = listItems(data);
      setProductReviewOrders((current) => {
        const items = appendUniqueReviewableOrders(current.items, pageItems);
        const total = Math.max(listTotal(data, pageItems), items.length);
        return {
          ...current,
          items,
          total,
          offset: pageItems.length > 0 ? offset + pageItems.length : total,
          loadingMore: false,
          error: ""
        };
      });
    } catch (error) {
      if (!isCurrentRequest()) return;
      setProductReviewOrders((current) => ({ ...current, loadingMore: false, error: error.message || "更多可评价订单加载失败" }));
    }
  }

  async function submitProductReview(event) {
    event.preventDefault();
    if (!token || !detailProduct?.id || reviewActionSubmittingRef.current) return;
    const productId = detailProduct.id;
    let reviewSession = detailReviewSessionRef.current;
    const isCurrentRequest = () => isCurrentDetailReviewRequest(productId, reviewSession);
    const orderId = selectedReviewOrderId;
    const content = reviewForm.content.trim();
    if (!orderId) {
      setReviewForm((current) => ({ ...current, error: "只有已完成且包含该商品的订单可以评价。" }));
      return;
    }
    if (!content) {
      setReviewForm((current) => ({ ...current, error: "请输入评价内容。" }));
      return;
    }
    reviewActionSubmittingRef.current = true;
    setReviewActionBusy(true);
    setReviewForm((current) => ({ ...current, orderId, action: "submit", error: "" }));
    try {
      await bbsApi.createMallProductReview(
        productId,
        {
          order_id: orderId,
          rating: Number(reviewForm.rating || 5),
          content
        },
        token
      );
      if (!isCurrentRequest()) return;
      reviewSession = ++detailReviewSessionRef.current;
      setProductReviewOrders((current) => {
        const items = current.items.filter((order) => reviewableOrderID(order) !== String(orderId));
        const total = Math.max(items.length, current.total - 1);
        return {
          ...current,
          items,
          total,
          offset: Math.min(current.offset, total),
          loading: false,
          loadingMore: false,
          error: ""
        };
      });
      const [reviewsResult, reviewableOrdersResult, myReviewsResult] = await Promise.allSettled([
        bbsApi.mallProductReviews(productId, { limit: SHOP_PRODUCT_REVIEW_PAGE_SIZE, offset: 0 }),
        bbsApi.mallReviewableOrders(productId, { limit: SHOP_REVIEWABLE_ORDER_PAGE_SIZE, offset: 0 }, token),
        bbsApi.mallReviews({ limit: SHOP_PRODUCT_REVIEW_PAGE_SIZE, offset: 0, product_id: productId }, token)
      ]);
      if (!isCurrentRequest()) return;
      if (reviewsResult.status === "fulfilled") {
        setProductReviews(productReviewPageState(reviewsResult.value));
      } else {
        setProductReviews((current) => ({ ...current, loading: false, error: reviewsResult.reason?.message || "评价已提交，公开评价列表刷新失败。" }));
      }
      if (reviewableOrdersResult.status === "fulfilled") {
        setProductReviewOrders(productReviewOrderPageState(reviewableOrdersResult.value));
      } else {
        setProductReviewOrders((current) => ({ ...current, loading: false, loadingMore: false, error: reviewableOrdersResult.reason?.message || "可评价订单刷新失败。" }));
      }
      if (myReviewsResult.status === "fulfilled") {
        setMyProductReviews(productReviewPageState(myReviewsResult.value));
      } else {
        setMyProductReviews((current) => ({ ...current, loading: false, error: myReviewsResult.reason?.message || "评价已提交，我的评价刷新失败。" }));
      }
      setReviewForm({ orderId: "", rating: 5, content: "", action: "", error: "" });
      setNotice("评价已提交，审核通过后会展示在商品详情。");
    } catch (error) {
      if (!isCurrentRequest()) return;
      setReviewForm((current) => ({ ...current, action: "", error: friendlyMallReviewError(error) }));
    } finally {
      reviewActionSubmittingRef.current = false;
      setReviewActionBusy(false);
    }
  }

  async function uploadReviewImage(event) {
    const file = event.target.files?.[0];
    event.target.value = "";
    if (!file || reviewActionSubmittingRef.current) return;
    if (!token) {
      setReviewForm((current) => ({ ...current, error: "请先登录后再上传图片。" }));
      return;
    }
    const productId = detailProduct?.id;
    const reviewSession = detailReviewSessionRef.current;
    const isCurrentRequest = () => isCurrentDetailReviewRequest(productId, reviewSession);
    reviewActionSubmittingRef.current = true;
    setReviewActionBusy(true);
    setReviewForm((current) => ({ ...current, action: "upload-image", error: "" }));
    try {
      const data = await bbsApi.uploadImage(file, token);
      if (!isCurrentRequest()) return;
      const imageUrl = data?.image_url || data?.imageUrl || data?.url || "";
      if (!imageUrl) {
        throw new Error("图片上传成功但未返回地址");
      }
      const nextContent = appendReviewImage(reviewForm.content, imageUrl);
      setReviewForm((current) => ({
        ...current,
        action: "",
        content: nextContent,
        error: ""
      }));
    } catch (error) {
      if (!isCurrentRequest()) return;
      setReviewForm((current) => ({ ...current, action: "", error: error.message || "图片上传失败" }));
    } finally {
      reviewActionSubmittingRef.current = false;
      setReviewActionBusy(false);
    }
  }

  function currentCheckoutAttemptIntent() {
    const receiver = fulfillment.receiver.trim();
    const phone = fulfillment.phone.trim();
    const address = formatFulfillmentAddress(fulfillment);
    return {
      mode: checkout.mode,
      items: checkoutLines.map((line) => ({ product_id: line.product?.id, quantity: toNumber(line.quantity) })),
      expected_original_credits: checkoutCost,
      coupon_code: checkoutCouponCode || undefined,
      receiver: checkoutRequiresShipping ? receiver : "",
      phone: checkoutRequiresShipping ? phone : "",
      address: checkoutRequiresShipping ? address : ""
    };
  }

  async function redeemProduct() {
    if (!token || checkoutLines.length === 0 || checkoutSubmittingRef.current) return;
    const requestToken = token;
    const requestUserId = auth?.user?.id;
    const shopSession = shopSessionRef.current;
    const isCurrentRequest = () => isCurrentShopSessionRequest(requestToken, shopSession);
    if (!isCurrentRequest()) return;
    const orderIntent = currentCheckoutAttemptIntent();
    const { receiver, phone, address } = orderIntent;
    if (checkoutRequiresShipping && (!receiver || !phone || !address)) {
      setCheckout((current) => ({ ...current, error: "请先补全收件人、联系电话和详细地址。" }));
      return;
    }
    if (checkoutHasStockIssue) {
      setCheckout((current) => ({ ...current, error: "购物车中有商品数量超过当前库存，请先调整数量。" }));
      return;
    }
    if (checkoutCouponCode && selectedCoupon && !selectedCouponAvailable) {
      setCheckout((current) => ({ ...current, error: "该优惠券当前不可用，请选择其他优惠券。" }));
      return;
    }
    if (checkoutCouponCode && selectedCoupon && !selectedCouponUsable) {
      setCheckout((current) => ({ ...current, error: `优惠券需满 ${couponMinOrderOf(selectedCoupon)} 积分可用。` }));
      return;
    }
    if (checkoutBalanceBlocked) {
      setCheckout((current) => ({ ...current, error: `积分不足，当前 ${balanceTotal}，还差 ${checkoutBalanceShortfall}。` }));
      return;
    }
    const requestID = ++checkoutRequestIdRef.current;
    checkoutSubmittingRef.current = requestID;
    const busyKey = checkout.mode === "cart" ? "cart" : checkoutLines[0]?.product?.id;
    setCheckoutActionBusy(true);
    setBusyProductId(busyKey);
    setNotice("");
    setCheckoutResultOrderId("");
    setCheckout((current) => ({ ...current, error: "" }));
    try {
      const orderPayload = { ...orderIntent };
      orderPayload.idempotency_key = checkoutAttemptKey({
        userId: requestUserId,
        intent: orderIntent
      });
      const orderData =
        checkout.mode === "cart"
          ? await bbsApi.checkoutMallCart(orderPayload, requestToken)
          : await bbsApi.createMallOrder(
              {
                ...orderPayload,
                items: checkoutLines.map((line) => ({ product_id: line.product.id, quantity: toNumber(line.quantity) }))
              },
              requestToken
            );
      const order = orderData?.order;
      if (!order?.id) {
        throw new Error("订单创建失败");
      }
      recordCheckoutAttemptOrder({ userId: requestUserId, intent: orderIntent, orderId: order.id });
      const settled = mallOrderPaymentSettled(order);
      const canPay = mallOrderCanPay(order);
      if (settled || !canPay) {
        clearCheckoutAttemptKey({ userId: requestUserId, intent: orderIntent });
      }
      if (!isCurrentRequest()) return;
      if (checkout.mode === "cart") applyCartData({ items: [], total: 0 });
      const paidCredits = toNumber(order.total_credits ?? order.totalCredits, checkoutPayableCost);
      const savedCredits = toNumber(order.discount_credits ?? order.discountCredits, checkoutDiscount);
      if (settled) {
        await refreshWallet();
        if (!isCurrentRequest()) return;
        if (checkoutCouponCode) {
          await refreshMyCoupons().catch(() => {});
          if (!isCurrentRequest()) return;
        }
        setCheckout({ product: null, items: [], mode: "", quantity: 1, couponCode: "", error: "" });
        setCheckoutResultOrderId(String(order.id));
        setNotice("订单已支付，积分流水已同步。");
        return;
      }
      if (!canPay) {
        throw new Error(`原订单${mallOrderStatusLabel(order.status)}，请重新确认兑换。`);
      }
      try {
        if (!isCurrentRequest()) return;
        await bbsApi.payMallOrder(
          order.id,
          {
            payment_method: "credits",
            idempotency_key: paymentAttemptKey("web-pay", order.id)
          },
          requestToken
        );
        if (!isCurrentRequest()) return;
        clearCheckoutAttemptKey({ userId: requestUserId, intent: orderIntent });
        await refreshWallet();
        if (!isCurrentRequest()) return;
        if (checkoutCouponCode) {
          await refreshMyCoupons().catch(() => {});
          if (!isCurrentRequest()) return;
        }
        setCheckout({ product: null, items: [], mode: "", quantity: 1, couponCode: "", error: "" });
        setCheckoutResultOrderId(String(order.id));
        setNotice(savedCredits > 0 ? `兑换成功，已优惠 ${savedCredits} 积分，实付 ${paidCredits} 积分。` : "兑换成功，订单已支付。");
      } catch (payError) {
        if (!isCurrentRequest()) return;
        await refreshWallet().catch(() => {});
        if (!isCurrentRequest()) return;
        if (checkoutCouponCode) {
          await refreshMyCoupons().catch(() => {});
          if (!isCurrentRequest()) return;
        }
        setCheckout({ product: null, items: [], mode: "", quantity: 1, couponCode: "", error: "" });
        setCheckoutResultOrderId(String(order.id));
        setNotice(`订单已创建，${friendlyMallCheckoutError(payError)}，可在个人工作台继续处理。`);
      }
    } catch (error) {
      if (!isCurrentRequest()) return;
      await syncCheckoutAfterMallError(error);
      if (!isCurrentRequest()) return;
      setCheckoutResultOrderId("");
      setCheckout((current) => ({ ...current, error: friendlyMallCheckoutError(error) }));
    } finally {
      if (checkoutSubmittingRef.current !== requestID) return;
      checkoutSubmittingRef.current = 0;
      setCheckoutActionBusy(false);
      setBusyProductId(null);
    }
  }

  function cancelCheckout() {
    if (checkoutSubmittingRef.current) return;
    setCheckout({ product: null, items: [], mode: "", quantity: 1, couponCode: "", error: "" });
  }

  return (
    <>
      <PageHero
        icon={ShoppingBag}
        eyebrow="商城"
        title="积分商城"
        description="用社区积分兑换数字权益、资料卡装饰和线下周边，订单与积分流水实时同步。"
        image={pageImages.商城}
        stats={[
          [state.loading ? "..." : String(state.total), "可兑换商品"],
          [token ? String(toNumber(balance?.total)) : "--", "当前积分"],
          [String(totalStock), state.items.length < state.total ? "已展示库存" : "库存"]
        ]}
      />
      {notice && <EmptyState title={notice} action={checkoutResultAction} />}
      {resumableCheckoutOrder && (
        <section className="panel content-block" aria-label="待完成支付订单">
          <BlockHeader icon={Activity} title="待完成支付" action="库存已为你保留" />
          <ListRow
            title={`${resumableCheckoutOrder.order_no || resumableCheckoutOrder.orderNo || `订单 #${resumableCheckoutOrder.id}`} · 待支付`}
            meta={`${orderAmountSummary(resumableCheckoutOrder)} · 支付失败或刷新后可继续完成，无需重新建单。`}
            actionLabel="继续支付"
            onAction={() => goOrders(resumableCheckoutOrder.id)}
          />
        </section>
      )}
      <section className="panel content-block shop-filter-panel">
        <BlockHeader icon={SlidersHorizontal} title="商品筛选" action={activeFilters ? "已筛选" : "全部商品"} />
        <form className="shop-search-form" onSubmit={submitFilters}>
          <label>
            <span>搜索商品</span>
            <input
              placeholder="输入商品名、SKU 或说明"
              value={keywordDraft}
              onChange={(event) => setKeywordDraft(event.target.value)}
            />
          </label>
          <button type="submit">
            <Search size={16} aria-hidden="true" />
            搜索
          </button>
          {activeFilters && (
            <button className="shop-filter-clear" type="button" onClick={clearFilters}>
              <X size={16} aria-hidden="true" />
              清除
            </button>
          )}
        </form>
        <div className="shop-category-filter">
          <label>
            <span>商品分类</span>
            <select aria-label="商城分类" value={filters.category} onChange={(event) => changeCategory(event.target.value)}>
              <option value="">全部分类</option>
              {categoryOptions.map((item) => (
                <option key={item.value} value={item.value}>
                  {item.label} ({item.count})
                </option>
              ))}
            </select>
          </label>
        </div>
      </section>
      {couponGuideVisible && (
        <section className="panel content-block coupon-guide-panel">
          <BlockHeader icon={BadgePercent} title="优惠券使用引导" action={checkoutCouponCode} />
          {!token && (
            <ListRow
              title={`优惠码 ${checkoutCouponCode} 已带入`}
              meta="登录并领取后，就能在结算时自动抵扣。"
              actionLabel="去登录"
              onAction={() => navigate("/user/signin")}
            />
          )}
          {token && myCoupons.loading && <ListRow title="正在校验券包" meta="正在确认这张优惠券是否可用" />}
          {token && !myCoupons.loading && !selectedCoupon && (
            <ListRow title="优惠码将由系统校验" meta="这张券暂未出现在券包中，仍可直接结算；若优惠码无效或不满足门槛，后端会返回明确提示。" actionLabel="刷新券包" onAction={refreshMyCoupons} />
          )}
          {selectedCoupon && (
            <>
              <div className="coupon-guide-summary">
                <span>{couponNameOf(selectedCoupon) || checkoutCouponCode}</span>
                <strong>-{couponDiscountOf(selectedCoupon)} 积分</strong>
                <small>{couponMinOrderOf(selectedCoupon) > 0 ? `满 ${couponMinOrderOf(selectedCoupon)} 积分可用` : "无门槛可用"} · {couponTimeText(selectedCoupon)}</small>
              </div>
              {!selectedCouponAvailable ? (
                <ListRow title="该优惠券已失效" meta="当前不能继续用于新订单。" />
              ) : couponGuideProducts.length === 0 ? (
                <ListRow
                  title="当前筛选下暂无满足门槛的商品"
                  meta="可清除筛选，或把多件商品加入购物车后再用券结算。"
                  actionLabel="清除筛选"
                  onAction={activeFilters ? clearFilters : undefined}
                />
              ) : (
                <div className="coupon-guide-list">
                  {couponGuideProducts.map((product) => {
                    const quantity = couponSuggestedQuantity(product, selectedCoupon);
                    const subtotal = toNumber(product.priceCredits) * quantity;
                    const discount = Math.min(couponDiscountOf(selectedCoupon), subtotal);
                    return (
                      <article key={product.id || product.sku}>
                        <img src={productImageOf(product)} alt="" />
                        <div>
                          <strong>{product.title}</strong>
                          <small>
                            {product.price} · 建议 x{quantity} · 小计 {subtotal} 积分
                          </small>
                        </div>
                        <span>可优惠 {discount} 积分</span>
                        <button type="button" disabled={checkoutBusy} onClick={() => openCouponCheckout(product)}>
                          带券兑换
                        </button>
                      </article>
                    );
                  })}
                </div>
              )}
            </>
          )}
        </section>
      )}
      {token && (
        <section className="panel content-block cart-panel">
          <BlockHeader icon={ShoppingBag} title="购物车" action={cartTotalQuantity > 0 ? `${cartTotalQuantity} 件 · ${cartTotalCredits} 积分` : "暂无商品"} />
          {cart.error && <p className="form-error">{cart.error}</p>}
          {cart.loading && <ListRow title="正在加载购物车" meta="请稍候" />}
          {!cart.loading && cartItems.length === 0 && <ListRow title="购物车暂无商品" meta="可以先把常用权益加入购物车，再一次性结算" />}
          {!cart.loading && cartItems.length > 0 && (
            <>
              <div className="cart-list">
                {cartItems.map((item) => {
                  const product = cartProductOf(item);
                  const productId = product?.id;
                  return (
                    <article key={productId || product?.sku}>
                      <img src={productImageOf(product)} alt="" />
                      <div>
                        <strong>{product?.title || "未命名商品"}</strong>
                        <small>
                          {product?.sku || "未配置 SKU"} · {toNumber(product?.priceCredits)} 积分 · 库存 {toNumber(product?.stock)}
                        </small>
                      </div>
                      <input
                        aria-label={`${product?.title || "商品"}数量`}
                        min="1"
                        max={toNumber(product?.stock)}
                        type="number"
                        value={cartItemQuantity(item)}
                        disabled={checkout.mode === "cart" || checkoutBusy || cart.action === `qty-${productId}`}
                        onChange={(event) => updateCartQuantity(item, event.target.value)}
                      />
                      <span>{cartItemSubtotal(item)} 积分</span>
                      <button type="button" disabled={checkout.mode === "cart" || checkoutBusy || cart.action === `remove-${productId}`} onClick={() => removeCartItem(item)}>
                        {cart.action === `remove-${productId}` ? "移除中" : "移除"}
                      </button>
                    </article>
                  );
                })}
              </div>
              <div className="cart-actions">
                <button type="button" disabled={cart.action === "clear" || checkout.mode === "cart" || checkoutBusy} onClick={clearCart}>
                  {cart.action === "clear" ? "清空中" : "清空购物车"}
                </button>
                <button type="button" disabled={checkout.mode === "cart" || checkoutBusy || Boolean(cart.action) || cart.loading || cartItems.length === 0} onClick={openCartCheckout}>
                  {busyProductId === "cart" ? "处理中" : "结算购物车"}
                </button>
              </div>
            </>
          )}
        </section>
      )}
      {token && (
        <section className="panel content-block favorite-products-panel">
          <BlockHeader icon={Star} title="收藏商品" action={favorites.total > 0 ? `${favorites.total} 件` : "暂无收藏"} onAction={reloadFavorites} />
          {favorites.error && <p className="form-error">{favorites.error}</p>}
          {favorites.loading && <ListRow title="正在加载收藏商品" meta="请稍候" />}
          {!favorites.loading && favoriteProducts.length === 0 && <ListRow title="暂无收藏商品" meta="点击商品卡片上的心形按钮，常用权益会出现在这里" />}
          {!favorites.loading && favoriteProducts.length > 0 && (
            <div className="favorite-product-list" aria-label="收藏商品列表">
              {favoriteProducts.map((product) => (
                <article key={product.id || product.sku}>
                  <img src={productImageOf(product)} alt="" />
                  <div>
                    <strong>{product.title}</strong>
                    <small>
                      {product.price} · 库存 {toNumber(product.stock)}
                    </small>
                  </div>
                  <button type="button" onClick={() => openProductDetail(product)}>
                    查看详情
                  </button>
                  <button type="button" disabled={checkout.mode === "cart" || checkoutBusy || cart.action === `add-${product.id}`} onClick={() => addToCart(product)}>
                    {cart.action === `add-${product.id}` ? "加入中" : "加购物车"}
                  </button>
                  <button type="button" disabled={favorites.action === `fav-${product.id}`} onClick={() => toggleProductFavorite(product)}>
                    {favorites.action === `fav-${product.id}` ? "处理中" : "取消收藏"}
                  </button>
                </article>
              ))}
            </div>
          )}
          {favoriteProducts.length > 0 && favorites.offset < favorites.total && (
            <div className="dashboard-history-more">
              <span>{favorites.loadingMore ? "正在加载更多收藏商品..." : "继续查看更多收藏商品。"}</span>
              <button type="button" aria-label="加载更多收藏商品" disabled={favorites.loadingMore} onClick={loadMoreFavorites}>
                {favorites.loadingMore ? "加载中" : "加载更多"}
              </button>
            </div>
          )}
        </section>
      )}
      <section className="panel content-block coupon-panel">
        <BlockHeader icon={BadgePercent} title="可领取优惠券" action={coupons.total > 0 ? `${coupons.total} 张` : "暂无优惠"} />
        {coupons.error && <p className="form-error">{coupons.error}</p>}
        {coupons.loading && <ListRow title="正在加载优惠券" meta="请稍候" />}
        {!coupons.loading && coupons.items.length === 0 && <ListRow title="暂无可用优惠券" meta="运营端投放优惠券后会展示在这里" />}
        {!coupons.loading && coupons.items.length > 0 && (
          <div className="coupon-list" aria-label="可领取优惠券列表">
            {coupons.items.map((coupon) => {
              const code = couponCodeOf(coupon);
              const couponId = couponIdOf(coupon);
              const claimed = token && (claimedCouponIds.has(String(couponId)) || claimedCouponCodes.has(code));
              const remaining = couponRemainingCount(coupon);
              const soldOut = couponTotalQuotaOf(coupon) > 0 && remaining <= 0 && !claimed;
              const busy = coupons.action === `claim-${couponId}`;
              return (
                <article className={`${claimed ? "is-selected" : ""} ${soldOut ? "is-disabled" : ""}`.trim()} key={couponId || code}>
                  <div>
                    <strong>{couponNameOf(coupon) || code || "优惠券"}</strong>
                    <small>{couponDescriptionOf(coupon) || couponTimeText(coupon)}</small>
                  </div>
                  <dl>
                    <div>
                      <dt>优惠</dt>
                      <dd>-{couponDiscountOf(coupon)} 积分</dd>
                    </div>
                    <div>
                      <dt>门槛</dt>
                      <dd>{couponMinOrderOf(coupon) > 0 ? `满 ${couponMinOrderOf(coupon)}` : "无门槛"}</dd>
                    </div>
                    <div>
                      <dt>剩余</dt>
                      <dd>{couponRemainingText(coupon)}</dd>
                    </div>
                  </dl>
                  <button type="button" disabled={!couponId || busy || claimed || soldOut} onClick={() => claimCoupon(coupon)}>
                    {busy ? "领取中" : claimed ? "已领取" : soldOut ? "已领完" : token ? "领取" : "登录领取"}
                  </button>
                </article>
              );
            })}
          </div>
        )}
        {coupons.items.length > 0 && coupons.offset < coupons.total && (
          <div className="dashboard-history-more">
            <span>{coupons.loadingMore ? "正在加载更多可领取优惠券..." : "继续查看更多可领取优惠券。"}</span>
            <button type="button" aria-label="加载更多可领取优惠券" disabled={coupons.loadingMore} onClick={loadMoreCoupons}>
              {coupons.loadingMore ? "加载中" : "加载更多"}
            </button>
          </div>
        )}
      </section>
      {token && (
        <section className="panel content-block coupon-panel">
          <BlockHeader icon={BadgePercent} title="我的优惠券" action={myCoupons.total > 0 ? `${myCoupons.total} 张` : "暂无已领取"} onAction={refreshMyCoupons} />
          {myCoupons.error && <p className="form-error">{myCoupons.error}</p>}
          {myCoupons.loading && <ListRow title="正在加载我的优惠券" meta="请稍候" />}
          {!myCoupons.loading && myClaimedCoupons.length === 0 && <ListRow title="暂无已领取优惠券" meta="先在上方领取，结算时再选择使用" />}
          {!myCoupons.loading && myClaimedCoupons.length > 0 && (
          <div className="coupon-list" aria-label="我的优惠券列表">
            {myClaimedCoupons.map((coupon) => {
                const code = couponCodeOf(coupon);
                const selected = checkoutCouponCode === code;
                const couponAvailable = mallCouponIsAvailable(coupon);
                const meetsThreshold = couponAvailable && (checkoutCost <= 0 || couponUsableForTotal(coupon, checkoutCost));
                return (
                  <article className={`${selected ? "is-selected" : ""} ${!meetsThreshold ? "is-disabled" : ""}`.trim()} key={coupon.id || couponIdOf(coupon) || code}>
                    <div>
                      <strong>{couponNameOf(coupon) || code || "优惠券"}</strong>
                      <small>{couponDescriptionOf(coupon) || couponTimeText(coupon)}</small>
                    </div>
                    <dl>
                      <div>
                        <dt>优惠</dt>
                        <dd>-{couponDiscountOf(coupon)} 积分</dd>
                      </div>
                      <div>
                        <dt>门槛</dt>
                        <dd>{couponMinOrderOf(coupon) > 0 ? `满 ${couponMinOrderOf(coupon)}` : "无门槛"}</dd>
                      </div>
                      <div>
                        <dt>状态</dt>
                        <dd>{couponAvailable ? "可使用" : "已失效"}</dd>
                      </div>
                    </dl>
                    <button type="button" disabled={!code || (!couponAvailable && !selected)} onClick={() => setCheckout((current) => ({ ...current, couponCode: selected ? "" : code, error: "" }))}>
                      {selected ? "取消选择" : couponAvailable ? "结算使用" : "已失效"}
                    </button>
                  </article>
              );
            })}
          </div>
        )}
        {myClaimedCoupons.length > 0 && myCoupons.offset < myCoupons.total && (
          <div className="dashboard-history-more">
            <span>{myCoupons.loadingMore ? "正在加载更多我的优惠券..." : "继续查看更多已领取优惠券。"}</span>
            <button type="button" aria-label="加载更多我的优惠券" disabled={myCoupons.loadingMore} onClick={loadMoreMyCoupons}>
              {myCoupons.loadingMore ? "加载中" : "加载更多"}
            </button>
          </div>
        )}
      </section>
      )}
      <section className="panel content-block">
        <BlockHeader icon={ShieldCheck} title="兑换信息" action={token ? "已登录" : "需登录"} />
        {token && addresses.length > 0 && (
          <div className="address-book">
            {addresses.map((address) => (
              <article
                className={`${String(address.id) === selectedAddressId ? "is-selected" : ""} ${String(address.id) === editingAddressId ? "is-editing" : ""}`.trim()}
                key={address.id}
              >
                <div>
                  <strong>
                    {address.receiver}
                    {(address.is_default || address.isDefault) && <span>默认</span>}
                  </strong>
                  <p>{address.phone}</p>
                  <small>{formatAddressLine(address)}</small>
                </div>
                <footer>
                  <button type="button" onClick={() => useAddress(address)}>
                    使用
                  </button>
                  <button type="button" disabled={addressAction !== ""} onClick={() => editAddress(address)}>
                    编辑
                  </button>
                  {!(address.is_default || address.isDefault) && (
                    <button type="button" disabled={addressAction === `default-${address.id}`} onClick={() => setDefaultAddress(address)}>
                      {addressAction === `default-${address.id}` ? "设置中" : "设默认"}
                    </button>
                  )}
                  <button type="button" disabled={addressAction === `delete-${address.id}`} onClick={() => deleteAddress(address)}>
                    {addressAction === `delete-${address.id}` ? "删除中" : "删除"}
                  </button>
                </footer>
              </article>
            ))}
          </div>
        )}
        {token && addresses.length > 0 && addressPage.offset < addressPage.total && (
          <div className="dashboard-history-more">
            <span>{addressPage.loadingMore ? "正在加载更多收货地址..." : addressPage.error || "继续查看更多收货地址。"}</span>
            <button type="button" aria-label="加载更多收货地址" disabled={addressPage.loadingMore} onClick={loadMoreAddresses}>
              {addressPage.loadingMore ? "加载中" : "加载更多"}
            </button>
          </div>
        )}
        <div className="settings-form compact-form">
          <label>
            <span>收件人</span>
            <input value={fulfillment.receiver} onChange={(event) => setFulfillment((current) => ({ ...current, receiver: event.target.value }))} />
          </label>
          <label>
            <span>联系电话</span>
            <input value={fulfillment.phone} onChange={(event) => setFulfillment((current) => ({ ...current, phone: event.target.value }))} />
          </label>
          <label>
            <span>省份</span>
            <input value={fulfillment.province} onChange={(event) => setFulfillment((current) => ({ ...current, province: event.target.value }))} />
          </label>
          <label>
            <span>城市</span>
            <input value={fulfillment.city} onChange={(event) => setFulfillment((current) => ({ ...current, city: event.target.value }))} />
          </label>
          <label>
            <span>区县</span>
            <input value={fulfillment.district} onChange={(event) => setFulfillment((current) => ({ ...current, district: event.target.value }))} />
          </label>
          <label>
            <span>邮编</span>
            <input value={fulfillment.postalCode} onChange={(event) => setFulfillment((current) => ({ ...current, postalCode: event.target.value }))} />
          </label>
          <label className="is-wide">
            <span>详细地址</span>
            <input value={fulfillment.detail} onChange={(event) => setFulfillment((current) => ({ ...current, detail: event.target.value }))} />
          </label>
        </div>
        {token && (
          <div className="address-book-actions">
            <button type="button" disabled={addressAction === "save"} onClick={saveAddress}>
              {addressAction === "save" ? "保存中" : editingAddressId ? "保存修改" : "保存为收货地址"}
            </button>
            {editingAddressId && (
              <button type="button" disabled={addressAction === "save"} onClick={cancelAddressEdit}>
                取消编辑
              </button>
            )}
          </div>
        )}
      </section>
      {detailProduct && (
        <section className="panel content-block product-detail-panel">
          <img src={detailProduct.image} alt="" />
          <div>
            <BlockHeader icon={ShoppingBag} title={detailProduct.title} action={detailProduct.badge} />
            <p>{detailProduct.desc}</p>
            <dl>
              <div>
                <dt>SKU</dt>
                <dd>{detailProduct.sku || "未配置"}</dd>
              </div>
              <div>
                <dt>分类</dt>
                <dd>{detailProduct.category || "未分类"}</dd>
              </div>
              <div>
                <dt>销量</dt>
                <dd>{detailProduct.salesCount}</dd>
              </div>
              <div>
                <dt>库存</dt>
                <dd>{detailProduct.stock}</dd>
              </div>
              {detailProduct.grantText && (
                <div>
                  <dt>权益</dt>
                  <dd>{detailProduct.grantText}</dd>
                </div>
              )}
            </dl>
            <footer>
              <strong>{detailProduct.price}</strong>
              <button type="button" onClick={closeProductDetail}>
                关闭
              </button>
              <button type="button" disabled={checkout.mode === "cart" || checkoutBusy || detailProduct.stock <= 0 || cart.action === `add-${detailProduct.id}`} onClick={() => addToCart(detailProduct)}>
                {cart.action === `add-${detailProduct.id}` ? "加入中" : "加购物车"}
              </button>
              <button type="button" disabled={!token || favorites.action === `fav-${detailProduct.id}`} onClick={() => toggleProductFavorite(detailProduct)}>
                {favoriteIds.has(String(detailProduct.id)) ? "取消收藏" : "收藏商品"}
              </button>
              <button type="button" disabled={checkoutBusy || detailProduct.stock <= 0} onClick={() => openCheckout(detailProduct)}>
                立即兑换
              </button>
            </footer>
            <div className="product-review-block">
              <BlockHeader icon={Star} title="商品评价" action={productReviews.loading ? "加载中" : `${productReviews.total} 条`} />
              {productReviews.error && productReviews.items.length === 0 && <p className="form-error">{productReviews.error}</p>}
              {!productReviews.loading && !productReviews.error && productReviews.items.length === 0 && <ListRow title="暂无评价" meta="完成兑换并审核通过后会展示使用体验" />}
              {productReviews.items.map((review) => {
                const reviewImages = markdownImageUrls(review.content);
                const reviewText = textWithoutMarkdownImages(review.content) || "未填写评价内容";
                return (
                  <div className="product-review-item" key={review.id}>
                    <div>
                      <strong>{reviewRatingText(review.rating)}</strong>
                      <span>用户 #{review.user_id || review.userId} · {timeAgoMillis(review.created_at || review.createdAt)}</span>
                    </div>
                    <p>{reviewText}</p>
                    {reviewImages.length > 0 && (
                      <div className="product-review-images">
                        {reviewImages.slice(0, 6).map((url, index) => (
                          <img src={url} alt="晒单图片" key={`${url}-${index}`} />
                        ))}
                      </div>
                    )}
                  </div>
                );
              })}
              {productReviews.items.length > 0 && productReviews.offset < productReviews.total && (
                <div className="dashboard-history-more">
                  <span>{productReviews.loadingMore ? "正在加载更多商品评价..." : productReviews.error || "继续查看更早的商品评价。"}</span>
                  <button type="button" aria-label="加载更多商品评价" disabled={productReviews.loadingMore} onClick={loadMoreProductReviews}>
                    {productReviews.loadingMore ? "加载中" : "加载更多"}
                  </button>
                </div>
              )}
              {productReviews.error && productReviews.items.length > 0 && productReviews.offset >= productReviews.total && <p className="form-error">{productReviews.error}</p>}
              {showMyProductReviews && (
                <section className="product-review-status-list">
                  <header>
                    <strong>我的评价进度</strong>
                    <span>{myProductReviews.loading ? "加载中" : `${myProductReviews.total} 条`}</span>
                  </header>
                  {myProductReviews.error && myProductReviews.items.length === 0 && <p className="form-error">{myProductReviews.error}</p>}
                  {myProductReviews.items.map((review) => {
                    const reviewImages = markdownImageUrls(review.content);
                    const reviewText = textWithoutMarkdownImages(review.content) || "未填写评价内容";
                    const statusKey = productReviewStatusKey(review.status);
                    return (
                      <article className="product-review-status-item" key={review.id}>
                        <div>
                          <strong>{reviewRatingText(review.rating)}</strong>
                          <span className={`product-review-status-badge status-${statusKey}`}>{productReviewStatusLabel(review.status)}</span>
                        </div>
                        <p>{reviewText}</p>
                        <small>
                          订单 #{review.order_id || review.orderId || "-"} · {timeAgoMillis(review.created_at || review.createdAt)}
                        </small>
                        {reviewImages.length > 0 && (
                          <div className="product-review-images">
                            {reviewImages.slice(0, 4).map((url, index) => (
                              <img src={url} alt="晒单图片" key={`${url}-${index}`} />
                            ))}
                          </div>
                        )}
                      </article>
                    );
                  })}
                  {myProductReviews.items.length > 0 && myProductReviews.offset < myProductReviews.total && (
                    <div className="dashboard-history-more">
                      <span>{myProductReviews.loadingMore ? "正在加载更多我的评价..." : myProductReviews.error || "继续查看更早的我的评价。"}</span>
                      <button type="button" aria-label="加载更多我的商品评价" disabled={myProductReviews.loadingMore} onClick={loadMoreMyProductReviews}>
                        {myProductReviews.loadingMore ? "加载中" : "加载更多"}
                      </button>
                    </div>
                  )}
                  {myProductReviews.error && myProductReviews.items.length > 0 && myProductReviews.offset >= myProductReviews.total && <p className="form-error">{myProductReviews.error}</p>}
                </section>
              )}
              {token && (
                <form className="product-review-form" onSubmit={submitProductReview}>
                  <label>
                    <span>可评价订单</span>
                    <select
                      value={selectedReviewOrderId}
                      disabled={reviewActionBusy || productReviewOrders.loading || reviewableOrders.length === 0}
                      onChange={(event) => setReviewForm((current) => ({ ...current, orderId: event.target.value, error: "" }))}
                    >
                      {productReviewOrders.loading ? (
                        <option value="">正在加载可评价订单</option>
                      ) : reviewableOrders.length === 0 ? (
                        <option value="">暂无已完成订单</option>
                      ) : (
                        reviewableOrders.map((order) => (
                          <option key={order.id} value={order.id}>
                            {order.order_no || order.orderNo || `订单 #${order.id}`}
                          </option>
                        ))
                      )}
                    </select>
                  </label>
                  {productReviewOrders.error && reviewableOrders.length === 0 && <p className="form-error">{productReviewOrders.error}</p>}
                  {reviewableOrders.length > 0 && productReviewOrders.offset < productReviewOrders.total && (
                    <div className="dashboard-history-more">
                      <span>{productReviewOrders.loadingMore ? "正在加载更多可评价订单..." : productReviewOrders.error || "继续加载更多可评价订单。"}</span>
                      <button type="button" aria-label="加载更多可评价订单" disabled={productReviewOrders.loadingMore} onClick={loadMoreProductReviewOrders}>
                        {productReviewOrders.loadingMore ? "加载中" : "加载更多"}
                      </button>
                    </div>
                  )}
                  {productReviewOrders.error && reviewableOrders.length > 0 && productReviewOrders.offset >= productReviewOrders.total && <p className="form-error">{productReviewOrders.error}</p>}
                  <label>
                    <span>评分</span>
                    <select
                      value={reviewForm.rating}
                      disabled={reviewActionBusy}
                      onChange={(event) => setReviewForm((current) => ({ ...current, rating: Number(event.target.value), error: "" }))}
                    >
                      {[5, 4, 3, 2, 1].map((rating) => (
                        <option key={rating} value={rating}>
                          {reviewRatingText(rating)}
                        </option>
                      ))}
                    </select>
                  </label>
                  <label className="product-review-content">
                    <span>评价内容</span>
                    <textarea
                      value={reviewForm.content}
                      disabled={reviewActionBusy}
                      maxLength={1000}
                      placeholder="说说兑换体验、使用效果或发货情况"
                      onChange={(event) => setReviewForm((current) => ({ ...current, content: event.target.value, error: "" }))}
                    />
                  </label>
                  <div className="product-review-media-tools">
                    <label>
                      <input accept="image/jpeg,image/png,image/gif,image/webp" className="sr-only" disabled={reviewActionBusy} type="file" onChange={uploadReviewImage} />
                      <span>{reviewForm.action === "upload-image" ? "图片上传中..." : "上传晒单图片"}</span>
                    </label>
                    <small>图片会插入评价正文，发布后在商品详情展示。</small>
                  </div>
                  {reviewForm.error && <p className="form-error">{reviewForm.error}</p>}
                  <button type="submit" disabled={reviewActionBusy || productReviewOrders.loading || productReviewOrders.loadingMore || reviewableOrders.length === 0}>
                    {reviewForm.action === "submit" ? "发布中" : "发布评价"}
                  </button>
                </form>
              )}
            </div>
          </div>
        </section>
      )}
      {checkoutLines.length > 0 && (
        <section className="panel content-block checkout-panel">
          <BlockHeader icon={ShoppingBag} title="确认兑换" action={`${checkoutPayableCost} 积分`} />
          {checkoutLines.map((line) => (
            <div className="checkout-summary" key={line.product.id}>
              <img src={line.product.image} alt="" />
              <div>
                <strong>{line.product.title}</strong>
                <p>{line.product.desc}</p>
                <span>
                  {line.product.price} · {line.product.stock} 库存
                </span>
              </div>
              {checkout.mode === "cart" ? (
                <span className="checkout-quantity">x {line.quantity}</span>
              ) : (
                <label>
                  数量
                  <input
                    min="1"
                    max={line.product.stock}
                    type="number"
                    value={checkout.quantity}
                    onChange={(event) =>
                      setCheckout((current) => ({
                        ...current,
                        quantity: Math.max(1, Math.min(current.product.stock, toNumber(event.target.value) || 1))
                      }))
                    }
                  />
                </label>
              )}
            </div>
          ))}
          <div className="checkout-address">
            {checkoutRequiresShipping ? (
              <>
                <span>{fulfillment.receiver || "未填写收件人"}</span>
                <span>{fulfillment.phone || "未填写电话"}</span>
                <span>{formatFulfillmentAddress(fulfillment) || "未填写地址"}</span>
              </>
            ) : (
              <span>{checkoutFulfillmentText}</span>
            )}
          </div>
          <div className="checkout-coupon">
            <label>
              <span>优惠码</span>
              <input
                value={checkout.couponCode || ""}
                placeholder="输入优惠码或从上方选择"
                onChange={(event) => setCheckout((current) => ({ ...current, couponCode: event.target.value.toUpperCase(), error: "" }))}
              />
            </label>
            <div className="checkout-coupon-status">
              {checkoutCouponMessage && <p className={`checkout-coupon-message status-${checkoutCouponState.status}`}>{checkoutCouponMessage}</p>}
              {checkoutCouponCode ? (
                <button
                  className="checkout-coupon-clear"
                  type="button"
                  aria-label="清除优惠码"
                  onClick={() => setCheckout((current) => ({ ...current, couponCode: "", error: "" }))}
                >
                  <X size={16} aria-hidden="true" />
                </button>
              ) : null}
            </div>
          </div>
          <div className={`checkout-wallet ${checkoutBalanceBlocked ? "is-insufficient" : ""} ${hasUnverifiedCouponCode ? "is-pending" : ""}`.trim()}>
            <span>
              当前积分 <strong>{balanceLoaded ? balanceTotal : "--"}</strong>
            </span>
            <span>
              商品合计 <strong>{checkoutCost}</strong>
            </span>
            <span>
              优惠 <strong>-{checkoutDiscount}</strong>
            </span>
            <span>
              应付积分 <strong>{checkoutPayableCost}</strong>
            </span>
            <span>
              {hasUnverifiedCouponCode ? "优惠码校验" : checkoutBalanceBlocked ? "还差积分" : "兑换后余额"} <strong>{hasUnverifiedCouponCode ? "以订单为准" : checkoutBalanceBlocked ? checkoutBalanceShortfall : checkoutRemaining}</strong>
            </span>
          </div>
          {checkout.error && <p className="form-error">{checkout.error}</p>}
          <div className="checkout-actions">
            <button type="button" disabled={checkoutBusy} onClick={cancelCheckout}>
              取消
            </button>
            {checkoutBalanceBlocked && (
              <button type="button" disabled={checkoutBusy} onClick={() => navigate("/tasks")}>
                去任务中心攒积分
              </button>
            )}
            <button
              type="button"
              disabled={
                checkoutBusy ||
                !canAttemptCouponCheckout ||
                checkoutBalanceBlocked
              }
              onClick={redeemProduct}
            >
              {checkoutBusy ? "处理中" : "确认兑换"}
            </button>
          </div>
        </section>
      )}
      {state.loading && <EmptyState title="正在加载商品..." />}
      {state.error && state.items.length === 0 && <EmptyState title="商品加载失败" description={state.error} />}
      {!state.loading && !state.error && products.length === 0 && <EmptyState title="暂无商品" description="运营端上架商品后会展示在这里。" />}
      {!state.loading && products.length > 0 && (
        <div className="shop-grid">
          {products.map((product) => (
            <ProductCard
              product={product}
              key={product.key}
              actionLabel={cart.action === `add-${product.id}` ? "加入中" : "加购物车"}
              actionDisabled={checkout.mode === "cart" || checkoutBusy || cart.action === `add-${product.id}` || product.stock <= 0}
              detailLabel="详情"
              favoriteActive={product.isFavorite}
              favoriteDisabled={favorites.action === `fav-${product.id}`}
              onAction={addToCart}
              onDetail={openProductDetail}
              onFavorite={token ? toggleProductFavorite : undefined}
            />
          ))}
        </div>
      )}
      {!state.loading && products.length > 0 && state.offset < state.total && (
        <div className="dashboard-history-more">
          <span>{state.loadingMore ? "正在加载更多商品..." : state.error || "继续浏览更多商品。"}</span>
          <button type="button" aria-label="加载更多商城商品" disabled={state.loadingMore} onClick={loadMoreProducts}>
            {state.loadingMore ? "加载中" : "加载更多"}
          </button>
        </div>
      )}
      <section className="panel content-block">
        <BlockHeader icon={Activity} title="最近订单" action={token ? "全部订单" : "登录查看"} onAction={() => goOrders()} />
        <div className="trend-bars">
          {orders.length === 0 && <ListRow title="暂无订单" meta={token ? "兑换后会显示最近订单" : "登录后查看订单历史"} />}
          {orders.map((order) => (
            <ListRow
              key={order.id}
              actionLabel="查看"
              title={`${order.order_no || order.orderNo || `订单 #${order.id}`} · ${mallOrderStatusLabel(order.status)}`}
              meta={`${orderAmountSummary(order)} · ${order.items?.length || 0} 件商品${formatOrderLogistics(order) ? ` · ${formatOrderLogistics(order)}` : ""}`}
              onAction={order.id ? () => goOrders(order.id) : undefined}
            />
          ))}
        </div>
      </section>
      <section className="panel content-block">
        <BlockHeader icon={ShieldCheck} title="兑换流程" action="积分支付" />
        <div className="step-row">
          <StepItem number="01" title="选择商品" desc="确认库存和兑换积分" />
          <StepItem number="02" title="生成订单" desc="库存会在下单时锁定" />
          <StepItem number="03" title="积分扣减" desc="支付成功后生成积分流水" />
        </div>
      </section>
    </>
  );
}

export function MorePage({ categories = [], hotTags = [] }) {
  const navigate = useNavigate();
  const [reloadKey, setReloadKey] = React.useState(0);
  const [state, setState] = React.useState({ links: [], tasks: [], leaderboard: [], loading: true, error: "", leaderboardError: "" });

  React.useEffect(() => {
    let alive = true;
    setState({ links: [], tasks: [], leaderboard: [], loading: true, error: "", leaderboardError: "" });
    Promise.allSettled([bbsApi.links({ limit: 6, offset: 0 }), bbsApi.tasks({ limit: 6, offset: 0 }), bbsApi.creditLeaderboard({ limit: 6 })]).then(([linkResult, taskResult, leaderboardResult]) => {
      if (!alive) return;
      const links = linkResult.status === "fulfilled" ? listItems(linkResult.value) : [];
      const tasks = taskResult.status === "fulfilled" ? listItems(taskResult.value) : [];
      const leaderboard = leaderboardResult.status === "fulfilled" ? listItems(leaderboardResult.value) : [];
      const failed = linkResult.status === "rejected" && taskResult.status === "rejected";
      setState({
        links,
        tasks,
        leaderboard,
        loading: false,
        error: failed ? "更多内容加载失败，请稍后重试。" : "",
        leaderboardError: leaderboardResult.status === "rejected" ? "排行榜加载失败，请稍后重试。" : ""
      });
    });
    return () => {
      alive = false;
    };
  }, [reloadKey]);

  const moreItems = [
    { title: "平台分类", desc: "当前开放的内容分区", icon: Grid3X3, value: `${categories.length} 个分类` },
    { title: "热门标签", desc: "用于发现内容和圈子", icon: Star, value: `${hotTags.length} 个标签` },
    { title: "资源入口", desc: "管理端维护的公开链接", icon: FolderOpen, value: state.loading ? "同步中" : `${state.links.length} 个资源` },
    { title: "成长任务", desc: "可参与的积分任务", icon: Trophy, value: state.loading ? "同步中" : `${state.tasks.length} 个任务` },
    { title: "积分排行榜", desc: "按当前积分余额实时排序", icon: Trophy, value: state.loading ? "同步中" : `${state.leaderboard.length} 位上榜用户` }
  ];
  const leaderboardRows = state.leaderboard.map((item) => leaderboardListRow(item, navigate));
  const rows = [
    ...state.links.map((item) => ({
      key: `link-${item.id || item.key}`,
      title: item.title || item.name || "资源入口",
      meta: item.description || safeExternalURL(item.url ?? item.URL) || "资源",
      actionHref: item.url ?? item.URL,
      actionIcon: ExternalLink,
      actionLabel: "访问"
    })),
    ...state.tasks.map((item) => ({
      key: `task-${item.id || item.key}`,
      title: item.title || item.name || "成长任务",
      meta: `${toNumber(item.reward_points ?? item.rewardPoints)} 积分 · ${item.description || "完成后获得成长值"}`,
      onAction: () => navigate("/tasks"),
      actionLabel: "查看任务"
    }))
  ];

  return (
    <>
      <PageHero
        icon={Grid3X3}
        eyebrow="更多"
        title="把低频但重要的入口收在一起"
        description="活动、公告、排行榜和工具箱集中展示，减少顶部导航负担，也保留扩展空间。"
        image={pageImages.更多}
        stats={[
          [String(categories.length), "开放分类"],
          [String(hotTags.length), "热门标签"],
          [state.loading ? "..." : String(state.links.length + state.tasks.length + state.leaderboard.length), "扩展入口"]
        ]}
      />
      <div className="more-grid">
        {moreItems.map((item) => (
          <MoreCard item={item} key={item.title} />
        ))}
      </div>
      <section className="panel content-block" aria-label="积分排行榜">
        <BlockHeader icon={Trophy} title="积分排行榜" action="刷新" onAction={() => setReloadKey((value) => value + 1)} />
        <div className="compact-list">
          {state.loading && <ListRow title="正在加载积分排行榜" meta="请稍候" />}
          {!state.loading && state.leaderboardError && <ListRow title={state.leaderboardError} meta="请稍后重试" />}
          {!state.loading && !state.leaderboardError && leaderboardRows.length === 0 && <ListRow title="暂无上榜用户" meta="完成成长任务或参与社区互动后可获得积分" />}
          {!state.loading && !state.leaderboardError && leaderboardRows.map((item) => <ListRow key={item.key} {...item} />)}
        </div>
      </section>
      <section className="panel content-block">
        <BlockHeader icon={MessageCircle} title="扩展入口" action="刷新" onAction={() => setReloadKey((value) => value + 1)} />
        <div className="compact-list">
          {state.loading && <ListRow title="正在加载扩展入口" meta="请稍候" />}
          {state.error && <ListRow title={state.error} meta="请稍后重试" />}
          {!state.loading && !state.error && rows.length === 0 && <ListRow title="暂无扩展入口" meta="资源或任务上线后会显示在这里" />}
          {!state.loading && !state.error && rows.map((item) => <ListRow key={item.key} {...item} />)}
        </div>
      </section>
    </>
  );
}

function leaderboardListRow(item, navigate) {
  const user = item?.user || {};
  const userId = String(item?.user_id ?? item?.userId ?? user?.id ?? "").trim();
  const nickname = String(user?.nickname || item?.nickname || "").trim();
  const username = String(user?.username || item?.username || "").trim();
  const rank = String(item?.rank ?? item?.Rank ?? "-").trim() || "-";
  const total = String(item?.total ?? item?.credits ?? item?.Total ?? 0).trim() || "0";
  const title = nickname || username || (userId ? `用户 #${userId}` : "社区用户");
  const account = username ? `@${username}` : "公开资料";
  const validUserId = /^\d+$/.test(userId) && userId !== "0";
  return {
    key: `leaderboard-${userId || rank}`,
    title: `#${rank} · ${title}`,
    meta: `${total} 积分 · ${account}`,
    onAction: validUserId ? () => navigate(`/user/${encodeURIComponent(userId)}`) : undefined,
    actionLabel: "查看资料"
  };
}

function homeContentItem(item, type) {
  const timestamp = item?.published_at || item?.publishedAt || item?.created_at || item?.createdAt;
  const sortAt = toNumber(timestamp);
  const tags = item?.tags || item?.tag_names || item?.tagNames || [];
  return {
    id: item?.id,
    type,
    sortAt,
    time: timeAgoMillis(timestamp),
    title: item?.title || `未命名${type}`,
    desc: `${type} · ${tags.length > 0 ? tags.slice(0, 3).join(" / ") : "暂无标签"}`
  };
}

function topicToQuestion(topic) {
  const tags = topic?.tags || topic?.tag_names || topic?.tagNames || [];
  const answers = toNumber(topic?.comment_count ?? topic?.commentCount);
  const bountyScore = toNumber(topic?.bounty_score ?? topic?.bountyScore);
  const qaStatus = topic?.qa_status || topic?.qaStatus || "";
  const acceptedCommentId = toId(topic?.accepted_comment_id ?? topic?.acceptedCommentId);
  const resolved = qaStatus === "resolved" || Boolean(acceptedCommentId && acceptedCommentId !== "0");
  return {
    id: topic?.id,
    title: topic?.title || "未命名求助",
    desc: topic?.body || topic?.summary || "暂无问题描述",
    status: resolved ? "已解决" : answers > 0 ? "有回复" : "待回答",
    bounty: bountyScore > 0 ? `${bountyScore} 积分悬赏` : "无悬赏",
    answers,
    tags
  };
}

function linkToResource(link, index) {
  const url = safeExternalURL(link.url ?? link.URL);
  return {
    key: link.id || link.key || index,
    title: link.title || link.name || "资源入口",
    desc: link.description || url || "暂无说明",
    type: "链接",
    meta: url || "已启用",
    icon: FileText,
    tags: [link.key || "resource", "资源"].filter(Boolean),
    url
  };
}

function mallProductToCard(product, index) {
  const stock = toNumber(product.stock);
  const priceCredits = toNumber(product.price_credits ?? product.priceCredits);
  const grantType = mallGrantTypeOf(product);
  const grantKey = mallGrantKeyOf(product);
  const grantText = mallGrantSnapshotText(product);
  return {
    id: product.id,
    key: product.id || product.sku || index,
    title: product.title || product.name || "积分商品",
    desc: product.description || "暂无商品说明",
    price: `${priceCredits} 积分`,
    priceCredits,
    badge: stock > 0 ? `${stock} 库存` : "已兑完",
    stock,
    sku: product.sku || "",
    category: product.category || "",
    grantType,
    grantKey,
    grantText,
    salesCount: toNumber(product.sales_count ?? product.salesCount),
    image: product.cover_url || product.coverUrl || workspacePhotos[index % workspacePhotos.length]
  };
}

function productRequiresShipping(product) {
  return mallProductRequiresShipping(product);
}

function checkoutDigitalFulfillmentText(lines = []) {
  const grantType = lines.map((line) => line.product?.grantType || mallGrantTypeOf(line.product)).find(Boolean);
  return `${mallGrantLabel(grantType || "digital")}在线发放，无需收货地址`;
}

function cartProductOf(item) {
  return mallProductToCard(item?.product || item?.Product || {}, 0);
}

function favoriteProductOf(item) {
  return item?.product || item?.Product || {};
}

function productFavoriteToCard(item, index) {
  const product = mallProductToCard(favoriteProductOf(item), index);
  return {
    ...product,
    favoriteCreatedAt: item?.created_at || item?.createdAt
  };
}

function couponSourceOf(coupon) {
  return coupon?.coupon || coupon?.Coupon || coupon || {};
}

function couponIdOf(coupon) {
  const source = couponSourceOf(coupon);
  return coupon?.coupon_id ?? coupon?.couponId ?? source?.id ?? source?.Id ?? coupon?.id ?? coupon?.Id ?? "";
}

function couponCodeOf(coupon) {
  const source = couponSourceOf(coupon);
  return String(coupon?.code || coupon?.Code || source?.code || source?.Code || "").trim().toUpperCase();
}

function couponNameOf(coupon) {
  const source = couponSourceOf(coupon);
  return source?.name || source?.Name || coupon?.name || coupon?.Name || "";
}

function couponDescriptionOf(coupon) {
  const source = couponSourceOf(coupon);
  return source?.description || source?.Description || coupon?.description || coupon?.Description || "";
}

function couponDiscountOf(coupon) {
  const source = couponSourceOf(coupon);
  return toNumber(coupon?.discount_credits ?? coupon?.discountCredits ?? source?.discount_credits ?? source?.discountCredits);
}

function couponMinOrderOf(coupon) {
  const source = couponSourceOf(coupon);
  return toNumber(coupon?.min_order_credits ?? coupon?.minOrderCredits ?? source?.min_order_credits ?? source?.minOrderCredits);
}

function couponTotalQuotaOf(coupon) {
  const source = couponSourceOf(coupon);
  return toNumber(coupon?.total_quota ?? coupon?.totalQuota ?? source?.total_quota ?? source?.totalQuota);
}

function couponClaimedOf(coupon) {
  const source = couponSourceOf(coupon);
  return toNumber(coupon?.claimed_count ?? coupon?.claimedCount ?? source?.claimed_count ?? source?.claimedCount);
}

function couponUsedOf(coupon) {
  const source = couponSourceOf(coupon);
  return toNumber(coupon?.used_count ?? coupon?.usedCount ?? source?.used_count ?? source?.usedCount);
}

function couponRemainingCount(coupon) {
  const total = couponTotalQuotaOf(coupon);
  if (total <= 0) return Infinity;
  return Math.max(0, total - couponClaimedOf(coupon));
}

function couponRemainingText(coupon) {
  const total = couponTotalQuotaOf(coupon);
  if (total <= 0) return "不限";
  return `${couponRemainingCount(coupon)} 张`;
}

function couponUsageSelectable(coupon) {
  const value = coupon?.status ?? coupon?.Status;
  if (value === undefined || value === null || value === "") return true;
  if (Number(value) === COUPON_USAGE_STATUS_CLAIMED) return true;
  const text = String(value).trim().toUpperCase();
  return text === "CLAIMED" || text === "COUPON_USAGE_STATUS_CLAIMED";
}

function couponUsableForTotal(coupon, totalCredits) {
  if (!coupon) return false;
  return mallCouponIsAvailable(coupon) && toNumber(totalCredits) >= couponMinOrderOf(coupon);
}

function couponSuggestedQuantity(product, coupon) {
  const price = toNumber(product?.priceCredits);
  const stock = toNumber(product?.stock);
  if (price <= 0 || stock <= 0) return 1;
  const minimum = Math.max(0, couponMinOrderOf(coupon));
  return Math.max(1, Math.min(stock, Math.ceil(minimum / price)));
}

function couponRecommendedProducts(products = [], coupon) {
  if (!coupon || couponDiscountOf(coupon) <= 0) return [];
  const minimum = Math.max(0, couponMinOrderOf(coupon));
  return products
    .filter((product) => {
      const price = toNumber(product?.priceCredits);
      const stock = toNumber(product?.stock);
      if (!product?.id || price <= 0 || stock <= 0) return false;
      return price * couponSuggestedQuantity(product, coupon) >= minimum;
    })
    .sort((left, right) => {
      const leftSubtotal = toNumber(left.priceCredits) * couponSuggestedQuantity(left, coupon);
      const rightSubtotal = toNumber(right.priceCredits) * couponSuggestedQuantity(right, coupon);
      if (leftSubtotal !== rightSubtotal) return leftSubtotal - rightSubtotal;
      if (toNumber(left.priceCredits) !== toNumber(right.priceCredits)) return toNumber(left.priceCredits) - toNumber(right.priceCredits);
      return String(left.title || "").localeCompare(String(right.title || ""), "zh-CN");
    });
}

function couponTimeText(coupon) {
  const source = couponSourceOf(coupon);
  const startsAt = toNumber(coupon?.starts_at ?? coupon?.startsAt ?? source?.starts_at ?? source?.startsAt);
  const endsAt = toNumber(coupon?.ends_at ?? coupon?.endsAt ?? source?.ends_at ?? source?.endsAt);
  if (!startsAt && !endsAt) return "长期有效";
  return `${formatCouponDate(startsAt) || "现在"} 至 ${formatCouponDate(endsAt) || "不限"}`;
}

function formatCouponDate(value) {
  const timestamp = toNumber(value);
  if (!timestamp) return "";
  const millis = timestamp > 9999999999 ? timestamp : timestamp * 1000;
  return new Date(millis).toLocaleDateString("zh-CN", { month: "2-digit", day: "2-digit" });
}

function orderAmountSummary(order) {
  const total = toNumber(order?.total_credits ?? order?.totalCredits);
  const original = toNumber(order?.original_credits ?? order?.originalCredits, total);
  const discount = toNumber(order?.discount_credits ?? order?.discountCredits);
  const couponCode = order?.coupon_code || order?.couponCode || "";
  if (discount > 0) {
    return `实付 ${total} 积分 · 已优惠 ${discount} 积分${couponCode ? ` · ${couponCode}` : ""} · 原价 ${original}`;
  }
  return `${total} 积分`;
}

function checkoutNoticeOpensOrder(notice) {
  const text = String(notice || "");
  return text.startsWith("兑换成功") || text.startsWith("订单已创建");
}

function cartItemQuantity(item) {
  return toNumber(item?.quantity);
}

function cartItemSubtotal(item) {
  const product = cartProductOf(item);
  return toNumber(product.priceCredits) * cartItemQuantity(item);
}

function productImageOf(product) {
  return product?.image || product?.cover_url || product?.coverUrl || workspacePhotos[0];
}

function checkoutCartLines(checkout) {
  if (checkout?.product) {
    return [
      {
        product: checkout.product,
        quantity: Math.max(1, Math.min(toNumber(checkout.product.stock), toNumber(checkout.quantity) || 1))
      }
    ];
  }
  if (!Array.isArray(checkout?.items)) return [];
  return checkout.items
    .map((item) => ({
      product: cartProductOf(item),
      quantity: cartItemQuantity(item)
    }))
    .filter((line) => line.product?.id && line.quantity > 0);
}

function addressToFulfillment(address) {
  return {
    receiver: address?.receiver || "",
    phone: address?.phone || "",
    province: address?.province || "",
    city: address?.city || "",
    district: address?.district || "",
    detail: address?.detail || formatAddressLine(address),
    postalCode: address?.postal_code || address?.postalCode || ""
  };
}

function formatAddressLine(address) {
  return [address?.province, address?.city, address?.district, address?.detail].filter(Boolean).join(" ").trim();
}

function emptyFulfillment(receiver = "") {
  return {
    receiver,
    phone: "",
    province: "",
    city: "",
    district: "",
    detail: "",
    postalCode: ""
  };
}

function formatFulfillmentAddress(fulfillment) {
  return [fulfillment.province, fulfillment.city, fulfillment.district, fulfillment.detail].filter(Boolean).join(" ").trim();
}

function mallCategoryOptions(items = []) {
  const counts = new Map();
  const configured = [];
  items.forEach((item) => {
    const configuredSlug = String(item.slug || item.value || "").trim();
    if (configuredSlug) {
      configured.push({
        value: configuredSlug,
        label: item.name || item.label || configuredSlug,
        count: toNumber(item.product_count ?? item.productCount ?? item.count)
      });
      return;
    }
    const category = String(item.category || "").trim();
    if (!category) return;
    counts.set(category, (counts.get(category) || 0) + 1);
  });
  if (configured.length > 0) {
    return configured.sort((left, right) => {
      if (left.count !== right.count) return right.count - left.count;
      return left.label.localeCompare(right.label, "zh-CN");
    });
  }
  return Array.from(counts.entries())
    .map(([value, count]) => ({ value, label: value, count }))
    .sort((left, right) => left.label.localeCompare(right.label, "zh-CN"));
}

function favoritePageState(data) {
  const items = listItems(data);
  return {
    items,
    total: Math.max(listTotal(data, items), items.length),
    offset: items.length,
    ids: favoriteProductIDSet(items),
    loading: false,
    loadingMore: false,
    error: ""
  };
}

function appendUniqueFavoriteItems(currentItems, pageItems) {
  const knownProductIDs = new Set(currentItems.map(favoriteProductID).filter(Boolean));
  return [
    ...currentItems,
    ...pageItems.filter((item) => {
      const productID = favoriteProductID(item);
      if (!productID || knownProductIDs.has(productID)) return false;
      knownProductIDs.add(productID);
      return true;
    })
  ];
}

function appendUniqueAddressItems(currentItems, pageItems) {
  const knownIDs = new Set(currentItems.map(addressListItemID).filter(Boolean));
  return [
    ...currentItems,
    ...pageItems.filter((item) => {
      const id = addressListItemID(item);
      if (!id || knownIDs.has(id)) return false;
      knownIDs.add(id);
      return true;
    })
  ];
}

function addressListItemID(address) {
  return String(address?.id ?? address?.ID ?? "").trim();
}

function favoriteProductIDSet(items) {
  return new Set(items.map(favoriteProductID).filter(Boolean));
}

function favoriteProductID(item) {
  return String(favoriteProductOf(item)?.id ?? item?.product_id ?? item?.productId ?? "").trim();
}

function couponPageState(data) {
  const items = listItems(data);
  return {
    items,
    total: Math.max(listTotal(data, items), items.length),
    offset: items.length,
    loading: false,
    loadingMore: false,
    error: ""
  };
}

function appendUniqueCouponItems(currentItems, pageItems) {
  const knownKeys = new Set(currentItems.map(couponListItemKey).filter(Boolean));
  return [
    ...currentItems,
    ...pageItems.filter((item) => {
      const key = couponListItemKey(item);
      if (!key || knownKeys.has(key)) return false;
      knownKeys.add(key);
      return true;
    })
  ];
}

function couponListItemKey(coupon) {
  const id = String(coupon?.id ?? coupon?.ID ?? "").trim();
  if (id) return `id:${id}`;
  const couponId = String(couponIdOf(coupon) || "").trim();
  if (couponId) return `coupon:${couponId}`;
  const code = couponCodeOf(coupon);
  return code ? `code:${code}` : "";
}

function productReviewPageState(data) {
  const items = listItems(data);
  return {
    items,
    total: Math.max(listTotal(data, items), items.length),
    offset: items.length,
    loading: false,
    loadingMore: false,
    error: ""
  };
}

function productReviewOrderPageState(data) {
  const items = listItems(data);
  return {
    items,
    total: Math.max(listTotal(data, items), items.length),
    offset: items.length,
    loading: false,
    loadingMore: false,
    error: ""
  };
}

function appendUniqueReviewableOrders(currentItems, pageItems) {
  const knownIDs = new Set(currentItems.map(reviewableOrderID).filter(Boolean));
  return [
    ...currentItems,
    ...pageItems.filter((item) => {
      const id = reviewableOrderID(item);
      if (!id || knownIDs.has(id)) return false;
      knownIDs.add(id);
      return true;
    })
  ];
}

function reviewableOrderID(order) {
  return String(order?.id ?? order?.ID ?? "").trim();
}

function appendUniqueProductReviews(currentItems, pageItems) {
  const knownIds = new Set(currentItems.map((item) => String(item?.id || "")).filter(Boolean));
  return [
    ...currentItems,
    ...pageItems.filter((item) => {
      const id = String(item?.id || "");
      if (!id || knownIds.has(id)) return false;
      knownIds.add(id);
      return true;
    })
  ];
}

function productReviewableOrders(orders = [], productId) {
  const normalizedProductId = String(productId || "");
  if (!normalizedProductId) return [];
  return orders.filter((order) => {
    if (!isCompletedOrder(order)) return false;
    return orderProductIds(order).has(normalizedProductId);
  });
}

function isCompletedOrder(order) {
  const normalized = String(order?.status || "").toUpperCase();
  return normalized === "6" || normalized === "COMPLETED";
}

function orderProductIds(order) {
  const ids = new Set();
  const items = Array.isArray(order?.items) ? order.items : [];
  items.forEach((item) => {
    const id = item?.product_id ?? item?.productId ?? item?.product?.id;
    if (id !== undefined && id !== null && id !== "") {
      ids.add(String(id));
    }
  });
  return ids;
}

function reviewOrderIdIn(orders = [], orderId) {
  const normalized = String(orderId || "");
  if (!normalized) return "";
  const match = orders.find((order) => String(order?.id || "") === normalized);
  return match ? String(match.id) : "";
}

function reviewRatingText(value) {
  const rating = Math.max(1, Math.min(5, toNumber(value, 5)));
  return `${"★".repeat(rating)}${"☆".repeat(5 - rating)}`;
}

function productReviewStatusKey(status) {
  const normalized = typeof status === "string" ? status.toUpperCase() : String(toNumber(status));
  if (normalized === "1" || normalized === "PENDING" || normalized === "PRODUCT_REVIEW_STATUS_PENDING") return "pending";
  if (normalized === "2" || normalized === "PUBLISHED" || normalized === "PRODUCT_REVIEW_STATUS_PUBLISHED") return "published";
  if (normalized === "3" || normalized === "HIDDEN" || normalized === "PRODUCT_REVIEW_STATUS_HIDDEN") return "hidden";
  return "unknown";
}

function productReviewStatusLabel(status) {
  const labels = {
    pending: "待审核",
    published: "已展示",
    hidden: "已隐藏",
    unknown: "状态未知"
  };
  return labels[productReviewStatusKey(status)];
}

function appendReviewImage(content, imageUrl) {
  const nextContent = appendMarkdownImage(content, imageUrl, "晒单图片");
  if (nextContent.length > 1000) {
    throw new Error("评价最多 1000 字，图片链接已达到上限。");
  }
  return nextContent;
}

function digitalEntitlementsOf(order) {
  const entitlements = order?.digital_entitlements ?? order?.digitalEntitlements ?? [];
  return Array.isArray(entitlements) ? entitlements : [];
}

function entitlementCode(entitlement) {
  return entitlement?.fulfillment_code || entitlement?.fulfillmentCode || "";
}

function entitlementStatus(entitlement) {
  return String(entitlement?.status || entitlement?.Status || "").trim().toUpperCase();
}

function entitlementRevokedAt(entitlement) {
  return entitlement?.revoked_at || entitlement?.revokedAt;
}

function entitlementRevoked(entitlement) {
  return entitlementStatus(entitlement) === "REVOKED" || Boolean(entitlementRevokedAt(entitlement));
}

function entitlementExpiresAt(entitlement) {
  return toNumber(entitlement?.expires_at ?? entitlement?.expiresAt);
}

function entitlementExpired(entitlement) {
  const expiresAt = entitlementExpiresAt(entitlement);
  return expiresAt > 0 && expiresAt <= Date.now();
}

function entitlementExpiryText(entitlement) {
  const expiresAt = entitlementExpiresAt(entitlement);
  if (!expiresAt) return "";
  const date = new Date(expiresAt);
  if (Number.isNaN(date.getTime())) return "";
  return `${entitlementExpired(entitlement) ? "已过期" : "有效至"} ${date.toLocaleDateString("zh-CN", { year: "numeric", month: "2-digit", day: "2-digit" })}`;
}

function digitalEntitlementSummary(order) {
  const entitlements = digitalEntitlementsOf(order);
  if (entitlements.length === 0) return "";
  const unavailableCount = entitlements.filter((item) => entitlementRevoked(item) || entitlementExpired(item)).length;
  const first = entitlements.find((item) => !entitlementRevoked(item) && !entitlementExpired(item)) || entitlements[0];
  const title = first.title || first.sku || "数字权益";
  const suffix = entitlements.length > 1 ? ` 等 ${entitlements.length} 项` : "";
  if (unavailableCount === entitlements.length) {
    return `${title}${suffix} · ${entitlementRevoked(first) ? "已撤销" : "已过期"}`;
  }
  if (unavailableCount > 0) {
    return `${title}${suffix} · ${unavailableCount} 项不可用`;
  }
  const code = entitlementCode(first);
  const expiry = entitlementExpiryText(first);
  return `${title}${suffix}${code ? ` · ${code}` : ""}${expiry ? ` · ${expiry}` : ""}`;
}

function formatOrderLogistics(order) {
  const entitlement = digitalEntitlementSummary(order);
  if (entitlement) {
    return `数字权益 ${entitlement}`;
  }
  const carrier = order?.shipping_carrier || order?.shippingCarrier;
  const trackingNo = order?.tracking_no || order?.trackingNo;
  if (carrier || trackingNo) {
    return [carrier, trackingNo].filter(Boolean).join(" / ");
  }
  const shippedAt = order?.shipped_at || order?.shippedAt;
  if (shippedAt) {
    return `已发货 ${timeAgoMillis(shippedAt)}`;
  }
  const completedAt = order?.completed_at || order?.completedAt;
  if (completedAt) {
    return `已完成 ${timeAgoMillis(completedAt)}`;
  }
  return "";
}
