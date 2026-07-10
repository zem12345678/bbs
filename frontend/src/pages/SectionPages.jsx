import React from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import {
  Activity,
  BadgePercent,
  CalendarDays,
  CircleHelp,
  Compass,
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
import { timeAgoMillis, toNumber } from "../lib/formatters";
import { paymentAttemptKey } from "../lib/idempotencyKeys";
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
  const visibleCircles = categories.length > 0 ? categories.slice(0, 6).map(categoryToCircle) : hotTags.slice(0, 6).map(tagToCircle);
  const weeklyPosts = categories.reduce((sum, item) => sum + toNumber(item.topicCount), 0);

  return (
    <>
      <PageHero
        icon={Users}
        eyebrow="圈子"
        title="按主题加入长期讨论"
        description="每个圈子都围绕稳定的技术主题沉淀帖子、资源和成员经验，适合持续共建。"
        image={pageImages.圈子}
        stats={[
          [String(visibleCircles.length), "开放圈子"],
          [String(hotTags.length), "热门标签"],
          [String(weeklyPosts), "分类内容"]
        ]}
      />
      {visibleCircles.length === 0 ? (
        <EmptyState title="暂无圈子数据" description="开放分类或热门标签后会自动生成圈子入口。" />
      ) : (
        <div className="circle-grid">
          {visibleCircles.map((circle) => (
            <CircleCard circle={circle} key={circle.name} />
          ))}
        </div>
      )}
      <section className="panel content-block">
        <BlockHeader icon={Rocket} title="圈子更新" action="查看分类" />
        <div className="compact-list">
          {visibleCircles.length === 0 && <ListRow title="暂无圈子更新" meta="发布内容并维护分类后会显示更新" />}
          {visibleCircles.slice(0, 4).map((circle) => (
            <ListRow key={circle.name} title={`${circle.name} 已开放讨论`} meta={`${circle.posts} 条内容 · ${circle.tags.slice(0, 2).join(" / ")}`} />
          ))}
        </div>
      </section>
    </>
  );
}

export function HelpPage() {
  const [state, setState] = React.useState({ items: [], total: 0, loading: true, error: "" });

  React.useEffect(() => {
    let alive = true;
    setState({ items: [], total: 0, loading: true, error: "" });
    bbsApi
      .listTopics({ limit: 8, offset: 0 })
      .then((data) => {
        if (!alive) return;
        const items = listItems(data);
        setState({ items, total: listTotal(data, items), loading: false, error: "" });
      })
      .catch((error) => {
        if (!alive) return;
        setState({ items: [], total: 0, loading: false, error: error.message || "求助内容加载失败" });
      });
    return () => {
      alive = false;
    };
  }, []);

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
      {state.error && <EmptyState title="求助内容加载失败" description={state.error} />}
      {!state.loading && !state.error && questions.length === 0 && <EmptyState title="暂无求助内容" description="发布话题后会出现在这里。" />}
      {!state.loading && !state.error && questions.length > 0 && (
        <div className="question-stack">
          {questions.map((question) => (
            <QuestionCard question={question} key={question.id} />
          ))}
        </div>
      )}
      <section className="panel content-block">
        <BlockHeader icon={Users} title="最新回复线索" action="去广场" />
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
  const [state, setState] = React.useState({ items: [], total: 0, loading: true, error: "" });

  React.useEffect(() => {
    let alive = true;
    setState({ items: [], total: 0, loading: true, error: "" });
    bbsApi
      .links({ limit: 12, offset: 0 })
      .then((data) => {
        if (!alive) return;
        const items = listItems(data);
        setState({ items, total: listTotal(data, items), loading: false, error: "" });
      })
      .catch((error) => {
        if (!alive) return;
        setState({ items: [], total: 0, loading: false, error: error.message || "资源加载失败" });
      });
    return () => {
      alive = false;
    };
  }, []);

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
      {state.error && <EmptyState title="资源加载失败" description={state.error} />}
      {!state.loading && !state.error && resources.length === 0 && <EmptyState title="暂无资源" description="在管理端维护友情链接后会展示在这里。" />}
      {!state.loading && !state.error && resources.length > 0 && (
        <div className="resource-grid">
          {resources.map((resource) => (
            <ResourceCard resource={resource} key={resource.key} />
          ))}
        </div>
      )}
      <section className="panel content-block">
        <BlockHeader icon={Activity} title="资源活跃度" action="全部资源" />
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
  const linkedProductId = searchParams.get("product_id") || "";
  const linkedReviewOrderId = searchParams.get("review_order_id") || "";
  const [state, setState] = React.useState({ items: [], total: 0, loading: true, error: "" });
  const [filters, setFilters] = React.useState({ keyword: "", category: "" });
  const [keywordDraft, setKeywordDraft] = React.useState("");
  const [categoryOptions, setCategoryOptions] = React.useState([]);
  const [balance, setBalance] = React.useState(null);
  const [orders, setOrders] = React.useState([]);
  const [cart, setCart] = React.useState({ items: [], total: 0, loading: false, error: "", action: "" });
  const [favorites, setFavorites] = React.useState({ items: [], total: 0, ids: new Set(), loading: false, error: "", action: "" });
  const [coupons, setCoupons] = React.useState({ items: [], total: 0, loading: true, error: "" });
  const [addresses, setAddresses] = React.useState([]);
  const [fulfillment, setFulfillment] = React.useState(() => emptyFulfillment(auth?.user?.nickname || ""));
  const [selectedAddressId, setSelectedAddressId] = React.useState("");
  const [detailProduct, setDetailProduct] = React.useState(null);
  const [productReviews, setProductReviews] = React.useState({ items: [], total: 0, loading: false, error: "" });
  const [myProductReviews, setMyProductReviews] = React.useState({ items: [], total: 0, loading: false, error: "" });
  const [productReviewOrders, setProductReviewOrders] = React.useState({ items: [], loading: false, error: "" });
  const [reviewForm, setReviewForm] = React.useState({ orderId: "", rating: 5, content: "", action: "", error: "" });
  const [checkout, setCheckout] = React.useState({ product: null, items: [], mode: "", quantity: 1, couponCode: "", error: "" });
  const [notice, setNotice] = React.useState("");
  const [addressAction, setAddressAction] = React.useState("");
  const [editingAddressId, setEditingAddressId] = React.useState("");
  const [busyProductId, setBusyProductId] = React.useState(null);

  React.useEffect(() => {
    let alive = true;
    setState({ items: [], total: 0, loading: true, error: "" });
    bbsApi
      .mallProducts({ limit: 24, offset: 0, keyword: filters.keyword, category: filters.category })
      .then((data) => {
        if (!alive) return;
        const items = listItems(data);
        setState({ items, total: listTotal(data, items), loading: false, error: "" });
      })
      .catch((error) => {
        if (!alive) return;
        setState({ items: [], total: 0, loading: false, error: error.message || "商品加载失败" });
      });
    return () => {
      alive = false;
    };
  }, [filters.category, filters.keyword]);

  React.useEffect(() => {
    let alive = true;
    bbsApi
      .mallCategories({ limit: 100, offset: 0 })
      .then((data) => {
        if (!alive) return;
        setCategoryOptions(mallCategoryOptions(listItems(data)));
      })
      .catch(() => bbsApi.mallProducts({ limit: 100, offset: 0 }))
      .then((data) => {
        if (!alive) return;
        if (data) {
          setCategoryOptions(mallCategoryOptions(listItems(data)));
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
    setCoupons((current) => ({ ...current, loading: true, error: "" }));
    bbsApi
      .mallCoupons({ limit: 12, offset: 0 })
      .then((data) => {
        if (!alive) return;
        const items = listItems(data);
        setCoupons({ items, total: listTotal(data, items), loading: false, error: "" });
      })
      .catch((error) => {
        if (!alive) return;
        setCoupons({ items: [], total: 0, loading: false, error: error.message || "优惠券加载失败" });
      });
    return () => {
      alive = false;
    };
  }, []);

  React.useEffect(() => {
    if (!token) {
      setBalance(null);
      setOrders([]);
      setCart({ items: [], total: 0, loading: false, error: "", action: "" });
      setFavorites({ items: [], total: 0, ids: new Set(), loading: false, error: "", action: "" });
      setAddresses([]);
      setSelectedAddressId("");
      setEditingAddressId("");
      return;
    }
    let alive = true;
    setCart((current) => ({ ...current, loading: true, error: "" }));
    setFavorites((current) => ({ ...current, loading: true, error: "" }));
    Promise.allSettled([
      bbsApi.creditBalance(token),
      bbsApi.mallOrders({ limit: 5, offset: 0 }, token),
      bbsApi.mallAddresses({ limit: 20, offset: 0 }, token),
      bbsApi.mallCart(token),
      bbsApi.mallProductFavorites({ limit: 20, offset: 0 }, token)
    ]).then(([balanceResult, orderResult, addressResult, cartResult, favoriteResult]) => {
        if (!alive) return;
        setBalance(balanceResult.status === "fulfilled" ? balanceResult.value?.balance || null : null);
        setOrders(orderResult.status === "fulfilled" ? listItems(orderResult.value) : []);
        if (addressResult.status === "fulfilled") {
          applyAddressList(listItems(addressResult.value));
        } else {
          setAddresses([]);
        }
        if (cartResult.status === "fulfilled") {
          applyCartData(cartResult.value);
        } else {
          setCart({ items: [], total: 0, loading: false, error: cartResult.reason?.message || "购物车加载失败", action: "" });
        }
        if (favoriteResult.status === "fulfilled") {
          applyFavoriteData(favoriteResult.value);
        } else {
          setFavorites({ items: [], total: 0, ids: new Set(), loading: false, error: favoriteResult.reason?.message || "收藏商品加载失败", action: "" });
        }
      });
    return () => {
      alive = false;
    };
  }, [token]);

  React.useEffect(() => {
    setFulfillment((current) => ({ ...current, receiver: current.receiver || auth?.user?.nickname || "" }));
  }, [auth?.user?.nickname]);

  React.useEffect(() => {
    if (!detailProduct?.id) {
      setProductReviews({ items: [], total: 0, loading: false, error: "" });
      setMyProductReviews({ items: [], total: 0, loading: false, error: "" });
      setProductReviewOrders({ items: [], loading: false, error: "" });
      setReviewForm({ orderId: "", rating: 5, content: "", action: "", error: "" });
      return;
    }
    let alive = true;
    setProductReviews({ items: [], total: 0, loading: true, error: "" });
    bbsApi
      .mallProductReviews(detailProduct.id, { limit: 10, offset: 0 })
      .then((data) => {
        if (!alive) return;
        const items = listItems(data);
        setProductReviews({ items, total: listTotal(data, items), loading: false, error: "" });
      })
      .catch((error) => {
        if (!alive) return;
        setProductReviews({ items: [], total: 0, loading: false, error: error.message || "评价加载失败" });
      });
    if (token) {
      setProductReviewOrders({ items: [], loading: true, error: "" });
      setMyProductReviews({ items: [], total: 0, loading: true, error: "" });
      bbsApi
        .mallReviewableOrders(detailProduct.id, { limit: 20, offset: 0 }, token)
        .then((data) => {
          if (!alive) return;
          setProductReviewOrders({ items: listItems(data), loading: false, error: "" });
        })
        .catch((error) => {
          if (!alive) return;
          setProductReviewOrders({ items: [], loading: false, error: error.message || "可评价订单加载失败" });
        });
      bbsApi
        .mallReviews({ limit: 10, offset: 0, product_id: detailProduct.id }, token)
        .then((data) => {
          if (!alive) return;
          const items = listItems(data);
          setMyProductReviews({ items, total: listTotal(data, items), loading: false, error: "" });
        })
        .catch((error) => {
          if (!alive) return;
          setMyProductReviews({ items: [], total: 0, loading: false, error: error.message || "我的评价加载失败" });
        });
    } else {
      setProductReviewOrders({ items: [], loading: false, error: "" });
      setMyProductReviews({ items: [], total: 0, loading: false, error: "" });
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

  const favoriteIds = favorites.ids || new Set();
  const products = state.items.map((item, index) => {
    const product = mallProductToCard(item, index);
    return { ...product, isFavorite: favoriteIds.has(String(product.id)) };
  });
  const favoriteProducts = favorites.items.map(productFavoriteToCard);
  const totalStock = state.items.reduce((sum, item) => sum + toNumber(item.stock), 0);
  const activeFilters = Boolean(filters.keyword || filters.category);
  const cartItems = Array.isArray(cart.items) ? cart.items : [];
  const cartTotalQuantity = cartItems.reduce((sum, item) => sum + cartItemQuantity(item), 0);
  const cartTotalCredits = cartItems.reduce((sum, item) => sum + cartItemSubtotal(item), 0);
  const checkoutLines = checkoutCartLines(checkout);
  const checkoutCost = checkoutLines.reduce((sum, line) => sum + toNumber(line.product?.priceCredits) * toNumber(line.quantity), 0);
  const checkoutCouponCode = String(checkout.couponCode || "").trim().toUpperCase();
  const selectedCoupon = coupons.items.find((item) => couponCodeOf(item) === checkoutCouponCode);
  const selectedCouponUsable = selectedCoupon ? couponUsableForTotal(selectedCoupon, checkoutCost) : false;
  const checkoutDiscount = selectedCouponUsable ? Math.min(couponDiscountOf(selectedCoupon), checkoutCost) : 0;
  const checkoutPayableCost = Math.max(0, checkoutCost - checkoutDiscount);
  const hasUnknownCouponCode = Boolean(checkoutCouponCode) && !selectedCoupon;
  const canAttemptCouponCheckout = !checkoutCouponCode || selectedCouponUsable;
  const balanceLoaded = Boolean(balance);
  const balanceTotal = balanceLoaded ? toNumber(balance?.total) : 0;
  const checkoutShortfall = balanceLoaded ? Math.max(0, checkoutPayableCost - balanceTotal) : 0;
  const checkoutRemaining = balanceLoaded ? Math.max(0, balanceTotal - checkoutPayableCost) : 0;
  const checkoutHasStockIssue = checkoutLines.some((line) => toNumber(line.quantity) <= 0 || toNumber(line.quantity) > toNumber(line.product?.stock));
  const checkoutRequiresShipping = checkoutLines.some((line) => productRequiresShipping(line.product));
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

  async function refreshWallet() {
    if (!token) return;
    const [balanceData, orderData] = await Promise.all([bbsApi.creditBalance(token), bbsApi.mallOrders({ limit: 5, offset: 0 }, token)]);
    setBalance(balanceData?.balance || null);
    setOrders(listItems(orderData));
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
    const items = listItems(data);
    setFavorites((current) => ({
      ...current,
      items,
      total: listTotal(data, items),
      ids: new Set(items.map((item) => String(favoriteProductOf(item)?.id || "")).filter(Boolean)),
      loading: false,
      error: "",
      action: ""
    }));
  }

  async function reloadCart() {
    if (!token) return [];
    setCart((current) => ({ ...current, loading: true, error: "" }));
    try {
      const data = await bbsApi.mallCart(token);
      const items = listItems(data);
      setCart({ items, total: listTotal(data, items), loading: false, error: "", action: "" });
      return items;
    } catch (error) {
      setCart((current) => ({ ...current, loading: false, error: error.message || "购物车加载失败", action: "" }));
      return [];
    }
  }

  async function reloadFavorites() {
    if (!token) return [];
    setFavorites((current) => ({ ...current, loading: true, error: "" }));
    try {
      const data = await bbsApi.mallProductFavorites({ limit: 20, offset: 0 }, token);
      const items = listItems(data);
      applyFavoriteData(data);
      return items;
    } catch (error) {
      setFavorites((current) => ({ ...current, loading: false, error: error.message || "收藏商品加载失败", action: "" }));
      return [];
    }
  }

  async function addToCart(product) {
    if (!token) {
      setNotice("请先登录后再加入购物车。");
      return;
    }
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
      setCart((current) => ({ ...current, action: "", error: error.message || "加入购物车失败" }));
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

  async function updateCartQuantity(item, quantity) {
    const product = cartProductOf(item);
    const productId = product?.id;
    if (!token || !productId) return;
    const nextQuantity = Math.max(1, Math.min(toNumber(product.stock), toNumber(quantity) || 1));
    setCart((current) => ({ ...current, action: `qty-${productId}`, error: "" }));
    try {
      const data = await bbsApi.setMallCartItem(productId, { quantity: nextQuantity }, token);
      applyCartData(data);
    } catch (error) {
      setCart((current) => ({ ...current, action: "", error: error.message || "更新购物车失败" }));
    }
  }

  async function removeCartItem(item) {
    const productId = cartProductOf(item)?.id;
    if (!token || !productId) return;
    setCart((current) => ({ ...current, action: `remove-${productId}`, error: "" }));
    try {
      const data = await bbsApi.removeMallCartItem(productId, token);
      applyCartData(data);
    } catch (error) {
      setCart((current) => ({ ...current, action: "", error: error.message || "移除购物车失败" }));
    }
  }

  async function clearCart() {
    if (!token || cartItems.length === 0) return;
    setCart((current) => ({ ...current, action: "clear", error: "" }));
    try {
      const data = await bbsApi.clearMallCart(token);
      applyCartData(data);
      setCheckout((current) => (current.mode === "cart" ? { product: null, items: [], mode: "", quantity: 1, couponCode: "", error: "" } : current));
    } catch (error) {
      setCart((current) => ({ ...current, action: "", error: error.message || "清空购物车失败" }));
    }
  }

  function applyAddressList(items) {
    setAddresses(items);
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
    const data = await bbsApi.mallAddresses({ limit: 20, offset: 0 }, token);
    const items = listItems(data);
    applyAddressList(items);
    return items;
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
    setCheckout((current) => ({ product, items: [], mode: "single", quantity: 1, couponCode: current.couponCode || "", error: "" }));
    closeProductDetail();
    setNotice("");
  }

  function openProductDetail(product) {
    setDetailProduct(product);
    if (product?.id) {
      setSearchParams({ product_id: String(product.id) });
    }
  }

  function closeProductDetail() {
    setDetailProduct(null);
    if (linkedProductId) {
      setSearchParams({}, { replace: true });
    }
  }

  function openCartCheckout() {
    if (!token) {
      setNotice("请先登录后再结算购物车。");
      return;
    }
    if (cartItems.length === 0) {
      setNotice("购物车暂无商品。");
      return;
    }
    setCheckout((current) => ({ product: null, items: cartItems, mode: "cart", quantity: 1, couponCode: current.couponCode || "", error: "" }));
    closeProductDetail();
    setNotice("");
  }

  function submitFilters(event) {
    event.preventDefault();
    setFilters((current) => ({ ...current, keyword: keywordDraft.trim() }));
  }

  function changeCategory(category) {
    setFilters((current) => ({ ...current, category }));
  }

  function clearFilters() {
    setKeywordDraft("");
    setFilters({ keyword: "", category: "" });
  }

  async function submitProductReview(event) {
    event.preventDefault();
    if (!token || !detailProduct?.id) return;
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
    setReviewForm((current) => ({ ...current, orderId, action: "submit", error: "" }));
    try {
      await bbsApi.createMallProductReview(
        detailProduct.id,
        {
          order_id: Number(orderId),
          rating: Number(reviewForm.rating || 5),
          content
        },
        token
      );
      setProductReviewOrders((current) => ({
        ...current,
        items: current.items.filter((order) => String(order.id) !== String(orderId)),
        loading: false,
        error: ""
      }));
      const [reviewsResult, reviewableOrdersResult, myReviewsResult] = await Promise.allSettled([
        bbsApi.mallProductReviews(detailProduct.id, { limit: 10, offset: 0 }),
        bbsApi.mallReviewableOrders(detailProduct.id, { limit: 20, offset: 0 }, token),
        bbsApi.mallReviews({ limit: 10, offset: 0, product_id: detailProduct.id }, token)
      ]);
      if (reviewsResult.status === "fulfilled") {
        const items = listItems(reviewsResult.value);
        setProductReviews({ items, total: listTotal(reviewsResult.value, items), loading: false, error: "" });
      } else {
        setProductReviews((current) => ({ ...current, loading: false, error: reviewsResult.reason?.message || "评价已提交，公开评价列表刷新失败。" }));
      }
      if (reviewableOrdersResult.status === "fulfilled") {
        setProductReviewOrders({ items: listItems(reviewableOrdersResult.value), loading: false, error: "" });
      } else {
        setProductReviewOrders((current) => ({ ...current, loading: false, error: reviewableOrdersResult.reason?.message || "可评价订单刷新失败。" }));
      }
      if (myReviewsResult.status === "fulfilled") {
        const items = listItems(myReviewsResult.value);
        setMyProductReviews({ items, total: listTotal(myReviewsResult.value, items), loading: false, error: "" });
      } else {
        setMyProductReviews((current) => ({ ...current, loading: false, error: myReviewsResult.reason?.message || "评价已提交，我的评价刷新失败。" }));
      }
      setReviewForm({ orderId: "", rating: 5, content: "", action: "", error: "" });
      setNotice("评价已提交，审核通过后会展示在商品详情。");
    } catch (error) {
      setReviewForm((current) => ({ ...current, action: "", error: error.message || "评价发布失败。" }));
    }
  }

  async function uploadReviewImage(event) {
    const file = event.target.files?.[0];
    event.target.value = "";
    if (!file) return;
    if (!token) {
      setReviewForm((current) => ({ ...current, error: "请先登录后再上传图片。" }));
      return;
    }
    setReviewForm((current) => ({ ...current, action: "upload-image", error: "" }));
    try {
      const data = await bbsApi.uploadImage(file, token);
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
      setReviewForm((current) => ({ ...current, action: "", error: error.message || "图片上传失败" }));
    }
  }

  async function redeemProduct() {
    if (checkoutLines.length === 0) return;
    const receiver = fulfillment.receiver.trim();
    const phone = fulfillment.phone.trim();
    const address = formatFulfillmentAddress(fulfillment);
    if (checkoutRequiresShipping && (!receiver || !phone || !address)) {
      setCheckout((current) => ({ ...current, error: "请先补全收件人、联系电话和详细地址。" }));
      return;
    }
    if (checkoutHasStockIssue) {
      setCheckout((current) => ({ ...current, error: "购物车中有商品数量超过当前库存，请先调整数量。" }));
      return;
    }
    if (checkoutCouponCode && selectedCoupon && !selectedCouponUsable) {
      setCheckout((current) => ({ ...current, error: `优惠券需满 ${couponMinOrderOf(selectedCoupon)} 积分可用。` }));
      return;
    }
    if (hasUnknownCouponCode) {
      setCheckout((current) => ({ ...current, error: "请从可用优惠券中选择，或清空未识别的优惠码。" }));
      return;
    }
    if (checkoutShortfall > 0) {
      setCheckout((current) => ({ ...current, error: `积分不足，当前 ${balanceTotal}，还差 ${checkoutShortfall}。` }));
      return;
    }
    const busyKey = checkout.mode === "cart" ? "cart" : checkoutLines[0]?.product?.id;
    setBusyProductId(busyKey);
    setNotice("");
    setCheckout((current) => ({ ...current, error: "" }));
    try {
      const orderPayload = {
        idempotency_key: `web-${checkout.mode || "single"}-${Date.now()}`,
        coupon_code: checkoutCouponCode || undefined,
        receiver: checkoutRequiresShipping ? receiver : "",
        phone: checkoutRequiresShipping ? phone : "",
        address: checkoutRequiresShipping ? address : ""
      };
      const orderData =
        checkout.mode === "cart"
          ? await bbsApi.checkoutMallCart(orderPayload, token)
          : await bbsApi.createMallOrder(
              {
                ...orderPayload,
                items: checkoutLines.map((line) => ({ product_id: line.product.id, quantity: toNumber(line.quantity) }))
              },
              token
            );
      const order = orderData?.order;
      if (!order?.id) {
        throw new Error("订单创建失败");
      }
      if (checkout.mode === "cart") {
        applyCartData({ items: [], total: 0 });
      }
      const paidCredits = toNumber(order.total_credits ?? order.totalCredits, checkoutPayableCost);
      const savedCredits = toNumber(order.discount_credits ?? order.discountCredits, checkoutDiscount);
      try {
        await bbsApi.payMallOrder(
          order.id,
          {
            payment_method: "credits",
            idempotency_key: paymentAttemptKey("web-pay", order.id)
          },
          token
        );
        await refreshWallet();
        setCheckout({ product: null, items: [], mode: "", quantity: 1, couponCode: "", error: "" });
        setNotice(savedCredits > 0 ? `兑换成功，已优惠 ${savedCredits} 积分，实付 ${paidCredits} 积分。` : "兑换成功，订单已支付。");
      } catch (payError) {
        await refreshWallet().catch(() => {});
        setCheckout({ product: null, items: [], mode: "", quantity: 1, couponCode: "", error: "" });
        setNotice(`订单已创建，${payError.message || "支付失败"}，可在个人工作台继续处理。`);
      }
    } catch (error) {
      setCheckout((current) => ({ ...current, error: error.message || "兑换失败，请稍后重试。" }));
    } finally {
      setBusyProductId(null);
    }
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
          [String(totalStock), "库存"]
        ]}
      />
      {notice && <EmptyState title={notice} />}
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
        <div className="shop-category-filter" role="tablist" aria-label="商城分类">
          <button className={filters.category === "" ? "is-active" : ""} type="button" onClick={() => changeCategory("")}>
            全部
          </button>
          {categoryOptions.map((item) => (
            <button
              className={filters.category === item.value ? "is-active" : ""}
              key={item.value}
              type="button"
              onClick={() => changeCategory(item.value)}
            >
              {item.label}
              <span>{item.count}</span>
            </button>
          ))}
        </div>
      </section>
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
                        onChange={(event) => updateCartQuantity(item, event.target.value)}
                      />
                      <span>{cartItemSubtotal(item)} 积分</span>
                      <button type="button" disabled={cart.action === `remove-${productId}`} onClick={() => removeCartItem(item)}>
                        {cart.action === `remove-${productId}` ? "移除中" : "移除"}
                      </button>
                    </article>
                  );
                })}
              </div>
              <div className="cart-actions">
                <button type="button" disabled={cart.action === "clear"} onClick={clearCart}>
                  {cart.action === "clear" ? "清空中" : "清空购物车"}
                </button>
                <button type="button" disabled={busyProductId === "cart" || cartItems.length === 0} onClick={openCartCheckout}>
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
            <div className="favorite-product-list">
              {favoriteProducts.map((product) => (
                <article key={product.id || product.sku}>
                  <img src={productImageOf(product)} alt="" />
                  <div>
                    <strong>{product.title}</strong>
                    <small>
                      {product.price} · 库存 {toNumber(product.stock)}
                    </small>
                  </div>
                  <button type="button" disabled={cart.action === `add-${product.id}`} onClick={() => addToCart(product)}>
                    {cart.action === `add-${product.id}` ? "加入中" : "加购物车"}
                  </button>
                  <button type="button" disabled={favorites.action === `fav-${product.id}`} onClick={() => toggleProductFavorite(product)}>
                    {favorites.action === `fav-${product.id}` ? "处理中" : "取消收藏"}
                  </button>
                </article>
              ))}
            </div>
          )}
        </section>
      )}
      <section className="panel content-block coupon-panel">
        <BlockHeader icon={BadgePercent} title="可用优惠券" action={coupons.total > 0 ? `${coupons.total} 张` : "暂无优惠"} />
        {coupons.error && <p className="form-error">{coupons.error}</p>}
        {coupons.loading && <ListRow title="正在加载优惠券" meta="请稍候" />}
        {!coupons.loading && coupons.items.length === 0 && <ListRow title="暂无可用优惠券" meta="运营端投放优惠券后会展示在这里" />}
        {!coupons.loading && coupons.items.length > 0 && (
          <div className="coupon-list">
            {coupons.items.map((coupon) => {
              const code = couponCodeOf(coupon);
              const selected = checkoutCouponCode === code;
              const meetsThreshold = checkoutCost <= 0 || couponUsableForTotal(coupon, checkoutCost);
              return (
                <article className={`${selected ? "is-selected" : ""} ${!meetsThreshold ? "is-disabled" : ""}`.trim()} key={coupon.id || code}>
                  <div>
                    <strong>{coupon.name || code || "优惠券"}</strong>
                    <small>{coupon.description || couponTimeText(coupon)}</small>
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
                  <button type="button" disabled={!code} onClick={() => setCheckout((current) => ({ ...current, couponCode: selected ? "" : code, error: "" }))}>
                    {selected ? "取消选择" : "结算使用"}
                  </button>
                </article>
              );
            })}
          </div>
        )}
      </section>
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
            </dl>
            <footer>
              <strong>{detailProduct.price}</strong>
              <button type="button" onClick={closeProductDetail}>
                关闭
              </button>
              <button type="button" disabled={detailProduct.stock <= 0 || cart.action === `add-${detailProduct.id}`} onClick={() => addToCart(detailProduct)}>
                {cart.action === `add-${detailProduct.id}` ? "加入中" : "加购物车"}
              </button>
              <button type="button" disabled={!token || favorites.action === `fav-${detailProduct.id}`} onClick={() => toggleProductFavorite(detailProduct)}>
                {favoriteIds.has(String(detailProduct.id)) ? "取消收藏" : "收藏商品"}
              </button>
              <button type="button" disabled={detailProduct.stock <= 0} onClick={() => openCheckout(detailProduct)}>
                立即兑换
              </button>
            </footer>
            <div className="product-review-block">
              <BlockHeader icon={Star} title="商品评价" action={productReviews.loading ? "加载中" : `${productReviews.total} 条`} />
              {productReviews.error && <p className="form-error">{productReviews.error}</p>}
              {!productReviews.loading && productReviews.items.length === 0 && <ListRow title="暂无评价" meta="完成兑换并审核通过后会展示使用体验" />}
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
              {showMyProductReviews && (
                <section className="product-review-status-list">
                  <header>
                    <strong>我的评价进度</strong>
                    <span>{myProductReviews.loading ? "加载中" : `${myProductReviews.total} 条`}</span>
                  </header>
                  {myProductReviews.error && <p className="form-error">{myProductReviews.error}</p>}
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
                </section>
              )}
              {token && (
                <form className="product-review-form" onSubmit={submitProductReview}>
                  <label>
                    <span>可评价订单</span>
                    <select
                      value={selectedReviewOrderId}
                      disabled={productReviewOrders.loading || reviewableOrders.length === 0}
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
                  {productReviewOrders.error && <p className="form-error">{productReviewOrders.error}</p>}
                  <label>
                    <span>评分</span>
                    <select
                      value={reviewForm.rating}
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
                      disabled={reviewForm.action === "upload-image"}
                      maxLength={1000}
                      placeholder="说说兑换体验、使用效果或发货情况"
                      onChange={(event) => setReviewForm((current) => ({ ...current, content: event.target.value, error: "" }))}
                    />
                  </label>
                  <div className="product-review-media-tools">
                    <label>
                      <input accept="image/jpeg,image/png,image/gif,image/webp" className="sr-only" disabled={reviewForm.action === "upload-image"} type="file" onChange={uploadReviewImage} />
                      <span>{reviewForm.action === "upload-image" ? "图片上传中..." : "上传晒单图片"}</span>
                    </label>
                    <small>图片会插入评价正文，发布后在商品详情展示。</small>
                  </div>
                  {reviewForm.error && <p className="form-error">{reviewForm.error}</p>}
                  <button type="submit" disabled={Boolean(reviewForm.action) || productReviewOrders.loading || reviewableOrders.length === 0}>
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
              <span>数字权益在线发放，无需收货地址</span>
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
            {checkoutCouponCode && selectedCoupon && (
              <p className={selectedCouponUsable ? "is-valid" : "is-invalid"}>
                {selectedCouponUsable
                  ? `${selectedCoupon.name || checkoutCouponCode} 已预估优惠 ${checkoutDiscount} 积分`
                  : `该优惠券需满 ${couponMinOrderOf(selectedCoupon)} 积分可用`}
              </p>
            )}
            {checkoutCouponCode && !selectedCoupon && <p className="is-invalid">未识别该优惠码，请从上方可用优惠券中选择。</p>}
          </div>
          <div className={`checkout-wallet ${checkoutShortfall > 0 ? "is-insufficient" : ""}`}>
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
              {checkoutShortfall > 0 ? "还差积分" : "兑换后余额"} <strong>{checkoutShortfall > 0 ? checkoutShortfall : checkoutRemaining}</strong>
            </span>
          </div>
          {checkout.error && <p className="form-error">{checkout.error}</p>}
          <div className="checkout-actions">
            <button type="button" onClick={() => setCheckout({ product: null, items: [], mode: "", quantity: 1, couponCode: "", error: "" })}>
              取消
            </button>
            <button
              type="button"
              disabled={
                busyProductId === (checkout.mode === "cart" ? "cart" : checkoutLines[0]?.product?.id) ||
                !canAttemptCouponCheckout ||
                checkoutShortfall > 0
              }
              onClick={redeemProduct}
            >
              {busyProductId === (checkout.mode === "cart" ? "cart" : checkoutLines[0]?.product?.id) ? "处理中" : "确认兑换"}
            </button>
          </div>
        </section>
      )}
      {state.loading && <EmptyState title="正在加载商品..." />}
      {state.error && <EmptyState title="商品加载失败" description={state.error} />}
      {!state.loading && !state.error && products.length === 0 && <EmptyState title="暂无商品" description="运营端上架商品后会展示在这里。" />}
      {!state.loading && !state.error && products.length > 0 && (
        <div className="shop-grid">
          {products.map((product) => (
            <ProductCard
              product={product}
              key={product.key}
              actionLabel={cart.action === `add-${product.id}` ? "加入中" : "加购物车"}
              actionDisabled={cart.action === `add-${product.id}` || product.stock <= 0}
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
      <section className="panel content-block">
        <BlockHeader icon={Activity} title="最近订单" action={token ? "全部订单" : "登录查看"} onAction={() => goOrders()} />
        <div className="trend-bars">
          {orders.length === 0 && <ListRow title="暂无订单" meta={token ? "兑换后会显示最近订单" : "登录后查看订单历史"} />}
          {orders.map((order) => (
            <ListRow
              key={order.id}
              actionLabel="查看"
              title={`${order.order_no || order.orderNo || `订单 #${order.id}`} · ${formatOrderStatus(order.status)}`}
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
  const [state, setState] = React.useState({ links: [], tasks: [], loading: true, error: "" });

  React.useEffect(() => {
    let alive = true;
    setState({ links: [], tasks: [], loading: true, error: "" });
    Promise.allSettled([bbsApi.links({ limit: 6, offset: 0 }), bbsApi.tasks({ limit: 6, offset: 0 })]).then(([linkResult, taskResult]) => {
      if (!alive) return;
      const links = linkResult.status === "fulfilled" ? listItems(linkResult.value) : [];
      const tasks = taskResult.status === "fulfilled" ? listItems(taskResult.value) : [];
      const failed = linkResult.status === "rejected" && taskResult.status === "rejected";
      setState({
        links,
        tasks,
        loading: false,
        error: failed ? "更多内容加载失败，请稍后重试。" : ""
      });
    });
    return () => {
      alive = false;
    };
  }, []);

  const moreItems = [
    { title: "平台分类", desc: "当前开放的内容分区", icon: Grid3X3, value: `${categories.length} 个分类` },
    { title: "热门标签", desc: "用于发现内容和圈子", icon: Star, value: `${hotTags.length} 个标签` },
    { title: "资源入口", desc: "管理端维护的公开链接", icon: FolderOpen, value: state.loading ? "同步中" : `${state.links.length} 个资源` },
    { title: "成长任务", desc: "可参与的积分任务", icon: Trophy, value: state.loading ? "同步中" : `${state.tasks.length} 个任务` }
  ];
  const rows = [
    ...state.links.map((item) => ({ key: `link-${item.id || item.key}`, title: item.title || item.name || "资源入口", meta: item.description || item.url || "资源" })),
    ...state.tasks.map((item) => ({ key: `task-${item.id || item.key}`, title: item.title || item.name || "成长任务", meta: `${toNumber(item.reward_points ?? item.rewardPoints)} 积分 · ${item.description || "完成后获得成长值"}` }))
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
          [state.loading ? "..." : String(state.links.length + state.tasks.length), "扩展入口"]
        ]}
      />
      <div className="more-grid">
        {moreItems.map((item) => (
          <MoreCard item={item} key={item.title} />
        ))}
      </div>
      <section className="panel content-block">
        <BlockHeader icon={MessageCircle} title="扩展入口" action="刷新" />
        <div className="compact-list">
          {state.loading && <ListRow title="正在加载扩展入口" meta="请稍候" />}
          {state.error && <ListRow title={state.error} meta="请稍后重试" />}
          {!state.loading && !state.error && rows.length === 0 && <ListRow title="暂无扩展入口" meta="资源或任务上线后会显示在这里" />}
          {!state.loading && !state.error && rows.map((item) => <ListRow key={item.key} title={item.title} meta={item.meta} />)}
        </div>
      </section>
    </>
  );
}

function tagToCircle(tag, index) {
  return {
    name: tag.name,
    desc: `围绕 ${tag.name} 的帖子、经验和资源沉淀`,
    members: `${Math.max(128, tag.count * 128).toLocaleString("zh-CN")}`,
    posts: String(tag.count),
    image: workspacePhotos[index % workspacePhotos.length],
    tags: [tag.name, "讨论", "精选"]
  };
}

function categoryToCircle(category, index) {
  return {
    name: category.name,
    desc: category.description || `围绕 ${category.name} 的长期讨论`,
    members: `${Math.max(128, category.topicCount * 96).toLocaleString("zh-CN")}`,
    posts: String(category.topicCount),
    image: workspacePhotos[index % workspacePhotos.length],
    tags: [category.slug || "category", "分类", "讨论"]
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
  return {
    id: topic?.id,
    title: topic?.title || "未命名话题",
    desc: topic?.body || topic?.summary || "暂无问题描述",
    status: answers > 0 ? "已回复" : "待回答",
    bounty: answers > 0 ? "已参与" : "待解答",
    answers,
    tags
  };
}

function linkToResource(link, index) {
  return {
    key: link.id || link.key || index,
    title: link.title || link.name || "资源入口",
    desc: link.description || link.url || "暂无说明",
    type: "链接",
    meta: link.url || "已启用",
    icon: FileText,
    tags: [link.key || "resource", "资源"].filter(Boolean),
    url: link.url
  };
}

function mallProductToCard(product, index) {
  const stock = toNumber(product.stock);
  const priceCredits = toNumber(product.price_credits ?? product.priceCredits);
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
    salesCount: toNumber(product.sales_count ?? product.salesCount),
    image: product.cover_url || product.coverUrl || workspacePhotos[index % workspacePhotos.length]
  };
}

function productRequiresShipping(product) {
  return String(product?.category || "").trim().toLowerCase() !== "digital";
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

function couponCodeOf(coupon) {
  return String(coupon?.code || coupon?.Code || "").trim().toUpperCase();
}

function couponDiscountOf(coupon) {
  return toNumber(coupon?.discount_credits ?? coupon?.discountCredits);
}

function couponMinOrderOf(coupon) {
  return toNumber(coupon?.min_order_credits ?? coupon?.minOrderCredits);
}

function couponTotalQuotaOf(coupon) {
  return toNumber(coupon?.total_quota ?? coupon?.totalQuota);
}

function couponClaimedOf(coupon) {
  return toNumber(coupon?.claimed_count ?? coupon?.claimedCount);
}

function couponUsedOf(coupon) {
  return toNumber(coupon?.used_count ?? coupon?.usedCount);
}

function couponRemainingText(coupon) {
  const total = couponTotalQuotaOf(coupon);
  if (total <= 0) return "不限";
  return `${Math.max(0, total - couponClaimedOf(coupon))} 张`;
}

function couponUsableForTotal(coupon, totalCredits) {
  if (!coupon) return false;
  return couponDiscountOf(coupon) > 0 && toNumber(totalCredits) >= couponMinOrderOf(coupon);
}

function couponTimeText(coupon) {
  const startsAt = toNumber(coupon?.starts_at ?? coupon?.startsAt);
  const endsAt = toNumber(coupon?.ends_at ?? coupon?.endsAt);
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

function formatOrderStatus(status) {
  const normalized = String(status || "").toUpperCase();
  switch (normalized) {
    case "1":
    case "PENDING_PAYMENT":
      return "待支付";
    case "2":
    case "PAYING":
      return "支付中";
    case "3":
    case "PAID":
      return "已支付";
    case "4":
    case "CANCELED":
      return "已取消";
    case "5":
    case "SHIPPED":
      return "已发货";
    case "6":
    case "COMPLETED":
      return "已完成";
    case "7":
    case "CLOSED":
      return "已关闭";
    case "8":
    case "REFUNDED":
      return "已退款";
    default:
      return "未知状态";
  }
}

function formatOrderLogistics(order) {
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
