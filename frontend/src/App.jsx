import React from "react";
import { BrowserRouter, Navigate, Route, Routes, useLocation, useNavigate } from "react-router-dom";
import { bbsApi } from "./api";
import FloatingRail from "./components/layout/FloatingRail.jsx";
import Header from "./components/layout/Header.jsx";
import { LeftColumn, RightColumn } from "./components/layout/PageColumns.jsx";
import { normalizeAuthResponse, persistAuth, readStoredAuth } from "./lib/authStorage";
import { normalizeCategoriesResponse, normalizeTagsResponse } from "./lib/catalog";
import { defaultPage, pageRoutes, pageToPath, pathToPage } from "./routes";

function lazyNamed(loader, exportName) {
  return React.lazy(() => loader().then((module) => ({ default: module[exportName] })));
}

const PlazaPage = React.lazy(() => import("./pages/PlazaPage.jsx"));
const MemberPage = React.lazy(() => import("./pages/MemberPage.jsx"));
const HomePage = lazyNamed(() => import("./pages/SectionPages.jsx"), "HomePage");
const CirclesPage = lazyNamed(() => import("./pages/SectionPages.jsx"), "CirclesPage");
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
  const [hotTags, setHotTags] = React.useState([]);
  const [categories, setCategories] = React.useState([]);

  function handleAuthSuccess(data) {
    const nextAuth = normalizeAuthResponse(data);
    setAuth(nextAuth);
    persistAuth(nextAuth);
  }

  function handleAuthUserUpdate(user) {
    if (!user) return;
    setAuth((current) => {
      if (!current?.accessToken) return current;
      const nextAuth = { ...current, user };
      persistAuth(nextAuth);
      return nextAuth;
    });
  }

  function handleLogout() {
    setAuth(null);
    persistAuth(null);
  }

  React.useEffect(() => {
    if (!auth?.accessToken) {
      return;
    }
    let alive = true;
    bbsApi
      .me(auth.accessToken)
      .then((data) => {
        if (!alive || !data?.user) return;
        const nextAuth = { ...auth, user: data.user };
        setAuth(nextAuth);
        persistAuth(nextAuth);
      })
      .catch(() => {
        if (!alive) return;
        setAuth(null);
        persistAuth(null);
      });
    return () => {
      alive = false;
    };
  }, [auth?.accessToken]);

  React.useEffect(() => {
    let alive = true;
    Promise.all([
      bbsApi.tags({ limit: 8 }).catch(() => ({ items: [] })),
      bbsApi.categories({ limit: 20 }).catch(() => ({ items: [] }))
    ])
      .then(([tagData, categoryData]) => {
        if (!alive) return;
        setHotTags(normalizeTagsResponse(tagData));
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

  const navigateToPage = React.useCallback(
    (page) => {
      navigate(pageToPath(page));
    },
    [navigate]
  );

  function handleSearch(query) {
    const keyword = query.trim();
    if (!keyword) {
      navigate("/search");
      return;
    }
    navigate(`/search?q=${encodeURIComponent(keyword)}`);
  }

  return (
    <div className="app">
      <Header
        activePage={activePage}
        auth={auth}
        onAuthSuccess={handleAuthSuccess}
        onCreate={() => navigate(auth ? "/dashboard/contents" : "/user/signin")}
        onDashboard={() => navigate(auth ? "/dashboard" : "/user/signin")}
        onLogout={handleLogout}
        onNavigate={navigateToPage}
        onSearch={handleSearch}
      />
      <React.Suspense fallback={<RouteLoading />}>
        <Routes>
          {pageRoutes.map(({ label, path }) => (
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
                  />
                ) : (
                  <SectionPage activePage={label} auth={auth} categories={categories} hotTags={hotTags} />
                )
              }
              key={label}
              path={path}
            />
          ))}
          <Route
            element={
              <FramedRoutePage activePage="广场" categories={categories} hotTags={hotTags}>
                <ContentListPage auth={auth} categories={categories} kind="topic" />
              </FramedRoutePage>
            }
            path="/topics"
          />
          <Route
            element={
              <FramedRoutePage activePage="广场" categories={categories} hotTags={hotTags}>
                <ContentListPage auth={auth} categories={categories} filter="category" kind="topic" />
              </FramedRoutePage>
            }
            path="/topics/category/:id"
          />
          <Route
            element={
              <FramedRoutePage activePage="广场" categories={categories} hotTags={hotTags}>
                <ContentListPage auth={auth} categories={categories} filter="tag" kind="topic" />
              </FramedRoutePage>
            }
            path="/topics/tag/:id"
          />
          <Route
            element={
              <FramedRoutePage activePage="广场" categories={categories} hotTags={hotTags}>
                <ContentListPage auth={auth} categories={categories} kind="article" />
              </FramedRoutePage>
            }
            path="/articles"
          />
          <Route
            element={
              <FramedRoutePage activePage="广场" categories={categories} hotTags={hotTags}>
                <ContentListPage auth={auth} categories={categories} filter="tag" kind="article" />
              </FramedRoutePage>
            }
            path="/articles/tag/:id"
          />
          <Route
            element={
              <FramedRoutePage activePage="广场" categories={categories} hotTags={hotTags}>
                <ContentDetailPage auth={auth} kind="topic" />
              </FramedRoutePage>
            }
            path="/topic/:id"
          />
          <Route
            element={
              <FramedRoutePage activePage="广场" categories={categories} hotTags={hotTags}>
                <ContentDetailPage auth={auth} kind="article" />
              </FramedRoutePage>
            }
            path="/article/:id"
          />
          <Route
            element={
              <FramedRoutePage activePage="广场" categories={categories} hotTags={hotTags}>
                <EditorPage auth={auth} categories={categories} kind="topic" />
              </FramedRoutePage>
            }
            path="/topic/create"
          />
          <Route
            element={
              <FramedRoutePage activePage="求助" categories={categories} hotTags={hotTags}>
                <EditorPage auth={auth} categories={categories} kind="question" />
              </FramedRoutePage>
            }
            path="/question/create"
          />
          <Route
            element={
              <FramedRoutePage activePage="求助" categories={categories} hotTags={hotTags}>
                <EditorPage auth={auth} categories={categories} edit kind="question" />
              </FramedRoutePage>
            }
            path="/question/edit/:id"
          />
          <Route
            element={
              <FramedRoutePage activePage="广场" categories={categories} hotTags={hotTags}>
                <EditorPage auth={auth} categories={categories} edit kind="topic" />
              </FramedRoutePage>
            }
            path="/topic/edit/:id"
          />
          <Route
            element={
              <FramedRoutePage activePage="广场" categories={categories} hotTags={hotTags}>
                <EditorPage auth={auth} categories={categories} kind="article" />
              </FramedRoutePage>
            }
            path="/article/create"
          />
          <Route
            element={
              <FramedRoutePage activePage="广场" categories={categories} hotTags={hotTags}>
                <EditorPage auth={auth} categories={categories} edit kind="article" />
              </FramedRoutePage>
            }
            path="/article/edit/:id"
          />
          <Route
            element={
              <FramedRoutePage activePage="广场" categories={categories} hotTags={hotTags}>
                <SearchPage auth={auth} categories={categories} />
              </FramedRoutePage>
            }
            path="/search"
          />
          <Route
            element={
              <FramedRoutePage activePage="会员" categories={categories} hotTags={hotTags}>
                <AuthCallbackPage auth={auth} onAuthSuccess={handleAuthSuccess} />
              </FramedRoutePage>
            }
            path="/auth/callback"
          />
          <Route
            element={
              <FramedRoutePage activePage="会员" categories={categories} hotTags={hotTags}>
                <AuthRoutePage auth={auth} mode="signin" onAuthSuccess={handleAuthSuccess} />
              </FramedRoutePage>
            }
            path="/user/signin"
          />
          <Route
            element={
              <FramedRoutePage activePage="会员" categories={categories} hotTags={hotTags}>
                <AuthRoutePage auth={auth} mode="signup" onAuthSuccess={handleAuthSuccess} />
              </FramedRoutePage>
            }
            path="/user/signup"
          />
          <Route
            element={
              <FramedRoutePage activePage="会员" categories={categories} hotTags={hotTags}>
                <ForgotPasswordPage />
              </FramedRoutePage>
            }
            path="/user/password/forgot"
          />
          <Route
            element={
              <FramedRoutePage activePage="会员" categories={categories} hotTags={hotTags}>
                <ResetPasswordPage />
              </FramedRoutePage>
            }
            path="/user/password/reset"
          />
          <Route
            element={
              <FramedRoutePage activePage="会员" categories={categories} hotTags={hotTags}>
                <EmailVerifyPage auth={auth} onAuthUserUpdate={handleAuthUserUpdate} />
              </FramedRoutePage>
            }
            path="/user/email/verify"
          />
          <Route
            element={
              <FramedRoutePage activePage="会员" categories={categories} hotTags={hotTags}>
                <UserRoutePage auth={auth} view="profile" />
              </FramedRoutePage>
            }
            path="/user/profile"
          />
          <Route
            element={
              <FramedRoutePage activePage="会员" categories={categories} hotTags={hotTags}>
                <UserRoutePage auth={auth} view="account" />
              </FramedRoutePage>
            }
            path="/user/profile/account"
          />
          <Route
            element={
              <FramedRoutePage activePage="会员" categories={categories} hotTags={hotTags}>
                <UserRoutePage auth={auth} view="favorites" />
              </FramedRoutePage>
            }
            path="/user/favorites"
          />
          <Route
            element={
              <FramedRoutePage activePage="会员" categories={categories} hotTags={hotTags}>
                <UserRoutePage auth={auth} view="messages" />
              </FramedRoutePage>
            }
            path="/user/messages"
          />
          <Route
            element={
              <FramedRoutePage activePage="会员" categories={categories} hotTags={hotTags}>
                <UserRoutePage auth={auth} view="scores" />
              </FramedRoutePage>
            }
            path="/user/scores"
          />
          <Route
            element={
              <FramedRoutePage activePage="会员" categories={categories} hotTags={hotTags}>
                <UserRoutePage auth={auth} view="profile" />
              </FramedRoutePage>
            }
            path="/user/:userId"
          />
          <Route
            element={
              <FramedRoutePage activePage="会员" categories={categories} hotTags={hotTags}>
                <UserRoutePage auth={auth} view="articles" />
              </FramedRoutePage>
            }
            path="/user/:userId/articles"
          />
          <Route
            element={
              <FramedRoutePage activePage="会员" categories={categories} hotTags={hotTags}>
                <UserRoutePage auth={auth} view="badges" />
              </FramedRoutePage>
            }
            path="/user/:userId/badges"
          />
          <Route
            element={
              <FramedRoutePage activePage="会员" categories={categories} hotTags={hotTags}>
                <UserRoutePage auth={auth} view="fans" />
              </FramedRoutePage>
            }
            path="/user/:userId/fans"
          />
          <Route
            element={
              <FramedRoutePage activePage="会员" categories={categories} hotTags={hotTags}>
                <UserRoutePage auth={auth} view="followed" />
              </FramedRoutePage>
            }
            path="/user/:userId/followed"
          />
          <Route
            element={
              <FramedRoutePage activePage="会员" categories={categories} hotTags={hotTags}>
                <UserDashboardPage auth={auth} onAuthUserUpdate={handleAuthUserUpdate} />
              </FramedRoutePage>
            }
            path="/dashboard"
          />
          <Route
            element={
              <FramedRoutePage activePage="会员" categories={categories} hotTags={hotTags}>
                <UserDashboardPage auth={auth} onAuthUserUpdate={handleAuthUserUpdate} />
              </FramedRoutePage>
            }
            path="/dashboard/:section"
          />
          {["links", "tasks", "about", "install", "redirect"].map((kind) => (
            <Route
              element={
                <FramedRoutePage activePage="更多" categories={categories} hotTags={hotTags}>
                  <AuxiliaryPage kind={kind} />
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
  );
}

function SectionPage({ activePage, auth, categories, hotTags }) {
  return (
    <main className="page-grid page-grid--section">
      <LeftColumn activePage={activePage} categories={categories} hotTags={hotTags} />
      <section className="feed page-view" aria-label={`${activePage}内容`}>
        {renderPage(activePage, auth, categories, hotTags)}
      </section>
      <RightColumn activePage={activePage} categories={categories} hotTags={hotTags} />
    </main>
  );
}

function FramedRoutePage({ activePage, categories, children, hotTags }) {
  return (
    <main className="page-grid page-grid--section">
      <LeftColumn activePage={activePage} categories={categories} hotTags={hotTags} />
      <section className="feed page-view" aria-label={`${activePage}扩展内容`}>
        {children}
      </section>
      <RightColumn activePage={activePage} categories={categories} hotTags={hotTags} />
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
      return <CirclesPage categories={categories} hotTags={hotTags} />;
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
