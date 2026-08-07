import React from "react";
import { BrowserRouter, Navigate, Route, Routes, useLocation, useNavigate } from "react-router-dom";
import { AUTH_INVALIDATED_EVENT, bbsApi, isUnauthorizedError } from "./api";
import SiteAnnouncements from "./components/SiteAnnouncements.jsx";
import FloatingRail from "./components/layout/FloatingRail.jsx";
import { LeftColumn, RightColumn } from "./components/layout/PageColumns.jsx";
import { normalizeAuthResponse, persistAuth, readStoredAuth } from "./lib/authStorage";
import { authInvalidationRedirect } from "./lib/authRedirect";
import { AppSessionContext } from "./lib/appSession";
import { normalizeCategoriesResponse, normalizeHashtagsResponse, normalizeTagsResponse } from "./lib/catalog";
import { defaultSiteConfig, normalizeSiteConfig } from "./lib/siteConfig";
import { defaultPage, pageRoutes, pageToPath, pathToPage } from "./routes";

function lazyNamed(loader, exportName) {
  return React.lazy(() => loader().then((module) => ({ default: module[exportName] })));
}

function setDocumentMeta(name, content) {
  let element = document.querySelector(`meta[name="${name}"]`);
  if (!element) {
    element = document.createElement("meta");
    element.setAttribute("name", name);
    document.head.appendChild(element);
  }
  element.setAttribute("content", content);
}

const PlazaPage = React.lazy(() => import("./pages/PlazaPage.jsx"));
const MemberPage = React.lazy(() => import("./pages/MemberPage.jsx"));
const HomePage = lazyNamed(() => import("./pages/SectionPages.jsx"), "HomePage");
const CirclesPage = lazyNamed(() => import("./pages/ChannelRoutes.jsx"), "CirclesPage");
const ChannelDetailPage = lazyNamed(() => import("./pages/ChannelRoutes.jsx"), "ChannelDetailPage");
const ChannelEditorPage = lazyNamed(() => import("./pages/ChannelRoutes.jsx"), "ChannelEditorPage");
const HelpPage = lazyNamed(() => import("./pages/SectionPages.jsx"), "HelpPage");
const ResourcesPage = lazyNamed(() => import("./pages/SectionPages.jsx"), "ResourcesPage");
const ShopPage = lazyNamed(() => import("./pages/SectionPages.jsx"), "ShopPage");
const MorePage = lazyNamed(() => import("./pages/SectionPages.jsx"), "MorePage");
const ContentListPage = lazyNamed(() => import("./pages/ContentRoutes.jsx"), "ContentListPage");
const ContentDetailPage = lazyNamed(() => import("./pages/ContentRoutes.jsx"), "ContentDetailPage");
const EditorPage = lazyNamed(() => import("./pages/ContentRoutes.jsx"), "EditorPage");
const SearchPage = lazyNamed(() => import("./pages/ContentRoutes.jsx"), "SearchPage");
const AuthCallbackPage = lazyNamed(() => import("./pages/AuthRoutes.jsx"), "AuthCallbackPage");
const AuthRoutePage = lazyNamed(() => import("./pages/AuthRoutes.jsx"), "AuthRoutePage");
const EmailVerifyPage = lazyNamed(() => import("./pages/AuthRoutes.jsx"), "EmailVerifyPage");
const ForgotPasswordPage = lazyNamed(() => import("./pages/AuthRoutes.jsx"), "ForgotPasswordPage");
const ResetPasswordPage = lazyNamed(() => import("./pages/AuthRoutes.jsx"), "ResetPasswordPage");
const AuxiliaryPage = lazyNamed(() => import("./pages/AuxiliaryPages.jsx"), "AuxiliaryPage");
const UserDashboardPage = lazyNamed(() => import("./pages/UserDashboardRoutes.jsx"), "UserDashboardPage");
const UserRoutePage = lazyNamed(() => import("./pages/UserRoutes.jsx"), "UserRoutePage");
const UserListDetailPage = lazyNamed(() => import("./pages/UserRoutes.jsx"), "UserListDetailPage");
const ChatPage = lazyNamed(() => import("./pages/ChatPage.jsx"), "ChatPage");

function App() {
  return (
    <BrowserRouter>
      <RoutedApp />
    </BrowserRouter>
  );
}

function RoutedApp() {
  const location = useLocation();
  const navigate = useNavigate();
  const activePage = pathToPage(location.pathname);
  const [auth, setAuth] = React.useState(readStoredAuth);
  const authRef = React.useRef(auth);
  const authRevisionRef = React.useRef(0);
  const locationRef = React.useRef(location);
  authRef.current = auth;
  locationRef.current = location;
  const [hotTags, setHotTags] = React.useState([]);
  const [categories, setCategories] = React.useState([]);
  const [siteConfig, setSiteConfig] = React.useState(defaultSiteConfig);

  function handleAuthSuccess(data) {
    const nextAuth = normalizeAuthResponse(data);
    authRevisionRef.current += 1;
    authRef.current = nextAuth;
    setAuth(nextAuth);
    persistAuth(nextAuth);
  }

  function handleAuthUserUpdate(user) {
    const currentAuth = authRef.current;
    if (!user || !currentAuth?.accessToken) return;
    const nextAuth = normalizeAuthResponse({ ...currentAuth, user });
    authRevisionRef.current += 1;
    authRef.current = nextAuth;
    setAuth(nextAuth);
    persistAuth(nextAuth);
  }

  const clearAuth = React.useCallback(() => {
    authRevisionRef.current += 1;
    authRef.current = null;
    setAuth(null);
    persistAuth(null);
  }, []);

  const handleLogout = React.useCallback(() => {
    const accessToken = authRef.current?.accessToken;
    clearAuth();
    if (accessToken) {
      void bbsApi.logout(accessToken).catch(() => {});
    }
  }, [clearAuth]);

  React.useEffect(() => {
    if (typeof window === "undefined") return undefined;
    function handleAuthInvalidated(event) {
      const failedToken = event?.detail?.accessToken;
      if (!failedToken || failedToken !== authRef.current?.accessToken) return;
      clearAuth();
      const currentLocation = locationRef.current;
      navigate(authInvalidationRedirect(`${currentLocation.pathname}${currentLocation.search}${currentLocation.hash}`), { replace: true });
    }
    window.addEventListener(AUTH_INVALIDATED_EVENT, handleAuthInvalidated);
    return () => window.removeEventListener(AUTH_INVALIDATED_EVENT, handleAuthInvalidated);
  }, [clearAuth, navigate]);

  React.useEffect(() => {
    if (!auth?.accessToken) {
      return;
    }
    let alive = true;
    const authRevision = authRevisionRef.current;
    bbsApi
      .me(auth.accessToken)
      .then((data) => {
        const currentAuth = authRef.current;
        if (!alive || !data?.user || authRevisionRef.current !== authRevision || currentAuth?.accessToken !== auth.accessToken) return;
        const nextAuth = normalizeAuthResponse({ ...currentAuth, user: data.user });
        authRef.current = nextAuth;
        setAuth(nextAuth);
        persistAuth(nextAuth);
      })
      .catch((error) => {
        if (!alive || !isUnauthorizedError(error) || authRef.current?.accessToken !== auth.accessToken) return;
        clearAuth();
      });
    return () => {
      alive = false;
    };
  }, [auth?.accessToken, clearAuth]);

  React.useEffect(() => {
    let alive = true;
    Promise.all([
      bbsApi.tags({ limit: 8 }).catch(() => ({ items: [] })),
      bbsApi.trendingHashtags({ limit: 8 }).catch(() => ({ items: [] })),
      bbsApi.categories({ limit: 20 }).catch(() => ({ items: [] }))
    ])
      .then(([tagData, hashtagData, categoryData]) => {
        if (!alive) return;
        const combinedTags = [...normalizeHashtagsResponse(hashtagData), ...normalizeTagsResponse(tagData)];
        const seenTags = new Set();
        setHotTags(combinedTags.filter((tag) => {
          const key = tag.name.toLocaleLowerCase();
          if (seenTags.has(key)) return false;
          seenTags.add(key);
          return true;
        }));
        setCategories(normalizeCategoriesResponse(categoryData));
      })
      .catch(() => {
        if (!alive) return;
        setHotTags([]);
        setCategories([]);
      });
    return () => {
      alive = false;
    };
  }, []);

  React.useEffect(() => {
    let alive = true;
    bbsApi
      .siteConfig()
      .then((data) => {
        if (!alive) return;
        const nextSiteConfig = normalizeSiteConfig(data);
        setSiteConfig(nextSiteConfig);
      })
      .catch(() => {});
    return () => {
      alive = false;
    };
  }, []);

  React.useEffect(() => {
    if (typeof document === "undefined") return;
    const siteName = siteConfig.siteName || defaultSiteConfig.siteName;
    document.title = activePage && activePage !== "首页" ? `${activePage} · ${siteName}` : siteName;
    setDocumentMeta("description", siteConfig.siteDescription || defaultSiteConfig.siteDescription);
    setDocumentMeta("keywords", siteConfig.seoKeywords || defaultSiteConfig.seoKeywords);
  }, [activePage, siteConfig]);

  return (
    <AppSessionContext.Provider value={{ auth, onLogout: handleLogout }}>
      <div className="app">
        <SiteAnnouncements />
        <React.Suspense fallback={<RouteLoading />}>
          <Routes>
          {pageRoutes.filter(({ key }) => key !== "chat").map(({ label, path }) => (
            <Route
              element={
                label === defaultPage ? (
                  <PlazaPage
                    activePage={label}
                    auth={auth}
                    categories={categories}
                    hotTags={hotTags}
                    LeftColumn={LeftColumn}
                    RightColumn={RightColumn}
                    siteConfig={siteConfig}
                  />
                ) : (
                  <SectionPage activePage={label} auth={auth} categories={categories} hotTags={hotTags} siteConfig={siteConfig} />
                )
              }
              key={label}
              path={path}
            />
          ))}
          <Route
            element={
              <FramedRoutePage activePage="广场" categories={categories} hotTags={hotTags} siteConfig={siteConfig}>
                <ContentListPage auth={auth} categories={categories} kind="topic" />
              </FramedRoutePage>
            }
            path="/topics"
          />
          <Route
            element={
              <FramedRoutePage activePage="广场" categories={categories} hotTags={hotTags} siteConfig={siteConfig}>
                <ContentListPage auth={auth} categories={categories} filter="category" kind="topic" />
              </FramedRoutePage>
            }
            path="/topics/category/:id"
          />
          <Route
            element={
              <FramedRoutePage activePage="广场" categories={categories} hotTags={hotTags} siteConfig={siteConfig}>
                <ContentListPage auth={auth} categories={categories} filter="tag" kind="topic" />
              </FramedRoutePage>
            }
            path="/topics/tag/:id"
          />
          <Route
            element={
              <FramedRoutePage activePage="圈子" categories={categories} hotTags={hotTags} siteConfig={siteConfig}>
                <ChannelEditorPage auth={auth} />
              </FramedRoutePage>
            }
            path="/circles/new"
          />
          <Route
            element={
              <FramedRoutePage activePage="圈子" categories={categories} hotTags={hotTags} siteConfig={siteConfig}>
                <ChannelEditorPage auth={auth} edit />
              </FramedRoutePage>
            }
            path="/circles/:id/edit"
          />
          <Route
            element={
              <FramedRoutePage activePage="圈子" categories={categories} hotTags={hotTags} siteConfig={siteConfig}>
                <ChannelDetailPage auth={auth} />
              </FramedRoutePage>
            }
            path="/circles/:id"
          />
          <Route
            element={
              <FramedRoutePage activePage="广场" categories={categories} hotTags={hotTags} siteConfig={siteConfig}>
                <ContentListPage auth={auth} categories={categories} kind="article" />
              </FramedRoutePage>
            }
            path="/articles"
          />
          <Route
            element={
              <FramedRoutePage activePage="广场" categories={categories} hotTags={hotTags} siteConfig={siteConfig}>
                <ContentListPage auth={auth} categories={categories} filter="tag" kind="article" />
              </FramedRoutePage>
            }
            path="/articles/tag/:id"
          />
          <Route
            element={
              <FramedRoutePage activePage="广场" categories={categories} hotTags={hotTags} siteConfig={siteConfig}>
                <ContentDetailPage auth={auth} kind="topic" />
              </FramedRoutePage>
            }
            path="/topic/:id"
          />
          <Route
            element={
              <FramedRoutePage activePage="广场" categories={categories} hotTags={hotTags} siteConfig={siteConfig}>
                <ContentDetailPage auth={auth} kind="article" />
              </FramedRoutePage>
            }
            path="/article/:id"
          />
          <Route
            element={
              <FramedRoutePage activePage="广场" categories={categories} hotTags={hotTags} siteConfig={siteConfig}>
                <EditorPage auth={auth} categories={categories} kind="topic" />
              </FramedRoutePage>
            }
            path="/topic/create"
          />
          <Route
            element={
              <FramedRoutePage activePage="求助" categories={categories} hotTags={hotTags} siteConfig={siteConfig}>
                <EditorPage auth={auth} categories={categories} kind="question" />
              </FramedRoutePage>
            }
            path="/question/create"
          />
          <Route
            element={
              <FramedRoutePage activePage="求助" categories={categories} hotTags={hotTags} siteConfig={siteConfig}>
                <EditorPage auth={auth} categories={categories} edit kind="question" />
              </FramedRoutePage>
            }
            path="/question/edit/:id"
          />
          <Route
            element={
              <FramedRoutePage activePage="广场" categories={categories} hotTags={hotTags} siteConfig={siteConfig}>
                <EditorPage auth={auth} categories={categories} edit kind="topic" />
              </FramedRoutePage>
            }
            path="/topic/edit/:id"
          />
          <Route
            element={
              <FramedRoutePage activePage="广场" categories={categories} hotTags={hotTags} siteConfig={siteConfig}>
                <EditorPage auth={auth} categories={categories} kind="article" />
              </FramedRoutePage>
            }
            path="/article/create"
          />
          <Route
            element={
              <FramedRoutePage activePage="广场" categories={categories} hotTags={hotTags} siteConfig={siteConfig}>
                <EditorPage auth={auth} categories={categories} edit kind="article" />
              </FramedRoutePage>
            }
            path="/article/edit/:id"
          />
          <Route
            element={
              <FramedRoutePage activePage="广场" categories={categories} hotTags={hotTags} siteConfig={siteConfig}>
                <SearchPage auth={auth} categories={categories} />
              </FramedRoutePage>
            }
            path="/search"
          />
          <Route element={<ChatPage auth={auth} onLogout={handleLogout} />} path="/chat" />
          <Route element={<ChatPage auth={auth} onLogout={handleLogout} />} path="/room/:roomNo" />
          <Route
            element={
              <FramedRoutePage activePage="会员" categories={categories} hotTags={hotTags} siteConfig={siteConfig}>
                <AuthCallbackPage auth={auth} onAuthSuccess={handleAuthSuccess} />
              </FramedRoutePage>
            }
            path="/auth/callback"
          />
          <Route
            element={
              <FramedRoutePage activePage="会员" categories={categories} hotTags={hotTags} siteConfig={siteConfig}>
                <AuthRoutePage auth={auth} mode="signin" onAuthSuccess={handleAuthSuccess} />
              </FramedRoutePage>
            }
            path="/user/signin"
          />
          <Route
            element={
              <FramedRoutePage activePage="会员" categories={categories} hotTags={hotTags} siteConfig={siteConfig}>
                <AuthRoutePage auth={auth} mode="signup" onAuthSuccess={handleAuthSuccess} />
              </FramedRoutePage>
            }
            path="/user/signup"
          />
          <Route
            element={
              <FramedRoutePage activePage="会员" categories={categories} hotTags={hotTags} siteConfig={siteConfig}>
                <ForgotPasswordPage />
              </FramedRoutePage>
            }
            path="/user/password/forgot"
          />
          <Route
            element={
              <FramedRoutePage activePage="会员" categories={categories} hotTags={hotTags} siteConfig={siteConfig}>
                <ResetPasswordPage onAuthInvalidated={clearAuth} />
              </FramedRoutePage>
            }
            path="/user/password/reset"
          />
          <Route
            element={
              <FramedRoutePage activePage="会员" categories={categories} hotTags={hotTags} siteConfig={siteConfig}>
                <EmailVerifyPage auth={auth} onAuthUserUpdate={handleAuthUserUpdate} />
              </FramedRoutePage>
            }
            path="/user/email/verify"
          />
          <Route
            element={
              <FramedRoutePage activePage="会员" categories={categories} hotTags={hotTags} siteConfig={siteConfig}>
                <UserRoutePage auth={auth} view="profile" />
              </FramedRoutePage>
            }
            path="/user/profile"
          />
          <Route
            element={
              <FramedRoutePage activePage="会员" categories={categories} hotTags={hotTags} siteConfig={siteConfig}>
                <UserRoutePage auth={auth} view="account" onAuthInvalidated={clearAuth} />
              </FramedRoutePage>
            }
            path="/user/profile/account"
          />
          <Route
            element={
              <FramedRoutePage activePage="会员" categories={categories} hotTags={hotTags} siteConfig={siteConfig}>
                <UserRoutePage auth={auth} view="favorites" />
              </FramedRoutePage>
            }
            path="/user/favorites"
          />
          <Route
            element={
              <FramedRoutePage activePage="会员" categories={categories} hotTags={hotTags} siteConfig={siteConfig}>
                <UserRoutePage auth={auth} view="likes" />
              </FramedRoutePage>
            }
            path="/user/likes"
          />
          <Route
            element={
              <FramedRoutePage activePage="会员" categories={categories} hotTags={hotTags} siteConfig={siteConfig}>
                <UserRoutePage auth={auth} view="lists" />
              </FramedRoutePage>
            }
            path="/user/lists"
          />
          <Route
            element={
              <FramedRoutePage activePage="会员" categories={categories} hotTags={hotTags} siteConfig={siteConfig}>
                <UserRoutePage auth={auth} view="messages" />
              </FramedRoutePage>
            }
            path="/user/messages"
          />
          <Route
            element={
              <FramedRoutePage activePage="会员" categories={categories} hotTags={hotTags} siteConfig={siteConfig}>
                <UserRoutePage auth={auth} view="scores" />
              </FramedRoutePage>
            }
            path="/user/scores"
          />
          <Route
            element={
              <FramedRoutePage activePage="会员" categories={categories} hotTags={hotTags} siteConfig={siteConfig}>
                <UserRoutePage auth={auth} view="safety" />
              </FramedRoutePage>
            }
            path="/user/safety"
          />
          <Route
            element={
              <FramedRoutePage activePage="会员" categories={categories} hotTags={hotTags} siteConfig={siteConfig}>
                <UserRoutePage auth={auth} view="profile" />
              </FramedRoutePage>
            }
            path="/user/:userId"
          />
          <Route
            element={
              <FramedRoutePage activePage="会员" categories={categories} hotTags={hotTags} siteConfig={siteConfig}>
                <UserRoutePage auth={auth} view="articles" />
              </FramedRoutePage>
            }
            path="/user/:userId/articles"
          />
          <Route
            element={
              <FramedRoutePage activePage="会员" categories={categories} hotTags={hotTags} siteConfig={siteConfig}>
                <UserRoutePage auth={auth} view="badges" />
              </FramedRoutePage>
            }
            path="/user/:userId/badges"
          />
          <Route
            element={
              <FramedRoutePage activePage="会员" categories={categories} hotTags={hotTags} siteConfig={siteConfig}>
                <UserRoutePage auth={auth} view="fans" />
              </FramedRoutePage>
            }
            path="/user/:userId/fans"
          />
          <Route
            element={
              <FramedRoutePage activePage="会员" categories={categories} hotTags={hotTags} siteConfig={siteConfig}>
                <UserRoutePage auth={auth} view="followed" />
              </FramedRoutePage>
            }
            path="/user/:userId/followed"
          />
          <Route
            element={
              <FramedRoutePage activePage="会员" categories={categories} hotTags={hotTags} siteConfig={siteConfig}>
                <UserRoutePage auth={auth} view="lists" />
              </FramedRoutePage>
            }
            path="/user/:userId/lists"
          />
          <Route
            element={
              <FramedRoutePage activePage="会员" categories={categories} hotTags={hotTags} siteConfig={siteConfig}>
                <UserRoutePage auth={auth} view="profile" />
              </FramedRoutePage>
            }
            path="/u/:username"
          />
          <Route
            element={
              <FramedRoutePage activePage="会员" categories={categories} hotTags={hotTags} siteConfig={siteConfig}>
                <UserRoutePage auth={auth} view="articles" />
              </FramedRoutePage>
            }
            path="/u/:username/articles"
          />
          <Route
            element={
              <FramedRoutePage activePage="会员" categories={categories} hotTags={hotTags} siteConfig={siteConfig}>
                <UserRoutePage auth={auth} view="badges" />
              </FramedRoutePage>
            }
            path="/u/:username/badges"
          />
          <Route
            element={
              <FramedRoutePage activePage="会员" categories={categories} hotTags={hotTags} siteConfig={siteConfig}>
                <UserRoutePage auth={auth} view="fans" />
              </FramedRoutePage>
            }
            path="/u/:username/fans"
          />
          <Route
            element={
              <FramedRoutePage activePage="会员" categories={categories} hotTags={hotTags} siteConfig={siteConfig}>
                <UserRoutePage auth={auth} view="followed" />
              </FramedRoutePage>
            }
            path="/u/:username/followed"
          />
          <Route
            element={
              <FramedRoutePage activePage="会员" categories={categories} hotTags={hotTags} siteConfig={siteConfig}>
                <UserRoutePage auth={auth} view="lists" />
              </FramedRoutePage>
            }
            path="/u/:username/lists"
          />
          <Route
            element={
              <FramedRoutePage activePage="会员" categories={categories} hotTags={hotTags} siteConfig={siteConfig}>
                <UserListDetailPage auth={auth} />
              </FramedRoutePage>
            }
            path="/user-lists/:listId"
          />
          <Route
            element={
              <FramedRoutePage activePage="会员" categories={categories} hotTags={hotTags} siteConfig={siteConfig}>
                <UserDashboardPage auth={auth} onAuthUserUpdate={handleAuthUserUpdate} />
              </FramedRoutePage>
            }
            path="/dashboard"
          />
          <Route
            element={
              <FramedRoutePage activePage="会员" categories={categories} hotTags={hotTags} siteConfig={siteConfig}>
                <UserDashboardPage auth={auth} onAuthUserUpdate={handleAuthUserUpdate} />
              </FramedRoutePage>
            }
            path="/dashboard/:section"
          />
          {["links", "tasks", "about", "install", "redirect"].map((kind) => (
            <Route
              element={
                <FramedRoutePage activePage="更多" categories={categories} hotTags={hotTags} siteConfig={siteConfig}>
                  <AuxiliaryPage auth={auth} kind={kind} siteConfig={siteConfig} />
                </FramedRoutePage>
              }
              key={kind}
              path={`/${kind}`}
            />
          ))}
          <Route element={<Navigate replace to={pageToPath(defaultPage)} />} path="*" />
          </Routes>
        </React.Suspense>
        <FloatingRail />
      </div>
    </AppSessionContext.Provider>
  );
}

function SectionPage({ activePage, auth, categories, hotTags, siteConfig }) {
  return (
    <main className="page-grid page-grid--section">
      <LeftColumn activePage={activePage} categories={categories} hotTags={hotTags} siteConfig={siteConfig} />
      <section className="feed page-view" aria-label={`${activePage}内容`}>
        {renderPage(activePage, auth, categories, hotTags)}
      </section>
      <RightColumn categories={categories} hotTags={hotTags} />
    </main>
  );
}

function FramedRoutePage({ activePage, categories, children, hotTags, siteConfig }) {
  return (
    <main className="page-grid page-grid--section">
      <LeftColumn activePage={activePage} categories={categories} hotTags={hotTags} siteConfig={siteConfig} />
      <section className="feed page-view" aria-label={`${activePage}扩展内容`}>
        {children}
      </section>
      <RightColumn categories={categories} hotTags={hotTags} />
    </main>
  );
}

function RouteLoading() {
  return (
    <main className="page-grid page-grid--section route-loading-shell">
      <section className="feed page-view route-loading" aria-live="polite">
        <span className="route-loading__dot" />
        正在加载页面…
      </section>
    </main>
  );
}

function renderPage(activePage, auth, categories, hotTags) {
  switch (activePage) {
    case "首页":
      return <HomePage categories={categories} hotTags={hotTags} />;
    case "圈子":
      return <CirclesPage auth={auth} />;
    case "求助":
      return <HelpPage />;
    case "资源":
      return <ResourcesPage />;
    case "商城":
      return <ShopPage auth={auth} />;
    case "会员":
      return <MemberPage auth={auth} categories={categories} />;
    case "更多":
      return <MorePage categories={categories} hotTags={hotTags} />;
    default:
      return <HomePage categories={categories} hotTags={hotTags} />;
  }
}

export default App;
