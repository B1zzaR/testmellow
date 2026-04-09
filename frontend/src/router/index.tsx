import { createBrowserRouter } from 'react-router-dom'
import { PrivateRoute } from './PrivateRoute'
import { AdminRoute } from './AdminRoute'
import { AppLayout } from '@/components/layout/AppLayout'
import { AdminLayout } from '@/components/layout/AdminLayout'
import { ErrorBoundary } from '@/components/ErrorBoundary'
import { LandingPage } from '@/pages/LandingPage'
import { PrivacyPolicyPage } from '@/pages/PrivacyPolicyPage'
import { UserAgreementPage } from '@/pages/UserAgreementPage'

// Auth pages
import { LoginPage } from '@/pages/auth/LoginPage'
import { RegisterPage } from '@/pages/auth/RegisterPage'

// User pages
import { DashboardPage } from '@/pages/user/DashboardPage'
import { SubscriptionsPage } from '@/pages/user/SubscriptionsPage'
import { RenewalPage } from '@/pages/user/RenewalPage'
import { ReferralsPage } from '@/pages/user/ReferralsPage'
import { ShopPage } from '@/pages/user/ShopPage'
import { TicketsPage } from '@/pages/user/TicketsPage'
import { TicketDetailPage } from '@/pages/user/TicketDetailPage'
import { PromoPage } from '@/pages/user/PromoPage'
import { BalancePage } from '@/pages/user/BalancePage'
import { ChangePasswordPage } from '../pages/user/ChangePasswordPage'
import { SettingsPage } from '@/pages/user/SettingsPage'
import { PaymentHistoryPage } from '@/pages/user/PaymentHistoryPage'
import { NotFoundPage } from '../pages/NotFoundPage'

// Admin pages
import { AdminDashboardPage } from '@/pages/admin/AdminDashboardPage'
import { AdminUsersPage } from '@/pages/admin/AdminUsersPage'
import { AdminUserDetailPage } from '@/pages/admin/AdminUserDetailPage'
import { AdminPromoPage } from '@/pages/admin/AdminPromoPage'
import { AdminTicketsPage } from '@/pages/admin/AdminTicketsPage'
import { AdminTicketDetailPage } from '@/pages/admin/AdminTicketDetailPage'
import { AdminPaymentsPage } from '@/pages/admin/AdminPaymentsPage'
import { AdminSubscriptionsPage } from '@/pages/admin/AdminSubscriptionsPage'
import { AdminReferralsPage } from '@/pages/admin/AdminReferralsPage'
import { AdminYADPage } from '@/pages/admin/AdminYADPage'
import { AdminNotificationsPage } from '@/pages/admin/AdminNotificationsPage'

export const router = createBrowserRouter([
  // Public routes
  { path: '/login', element: <LoginPage /> },
  { path: '/register', element: <RegisterPage /> },
  { path: '/', element: <LandingPage /> },
  { path: '/PrivacyPolicy', element: <PrivacyPolicyPage /> },
  { path: '/UserAgreement', element: <UserAgreementPage /> },

  // Protected user routes
  {
    element: <PrivateRoute />,
    children: [
      {
        element: <AppLayout />,
        children: [
          { path: '/dashboard', element: <ErrorBoundary><DashboardPage /></ErrorBoundary> },
          { path: '/subscriptions', element: <ErrorBoundary><SubscriptionsPage /></ErrorBoundary> },
          { path: '/subscriptions/renew', element: <ErrorBoundary><RenewalPage /></ErrorBoundary> },
          { path: '/referrals', element: <ErrorBoundary><ReferralsPage /></ErrorBoundary> },
          { path: '/shop', element: <ErrorBoundary><ShopPage /></ErrorBoundary> },
          { path: '/tickets', element: <ErrorBoundary><TicketsPage /></ErrorBoundary> },
          { path: '/tickets/:id', element: <ErrorBoundary><TicketDetailPage /></ErrorBoundary> },
          { path: '/promo', element: <ErrorBoundary><PromoPage /></ErrorBoundary> },
          { path: '/balance', element: <ErrorBoundary><BalancePage /></ErrorBoundary> },
          { path: '/settings', element: <ErrorBoundary><SettingsPage /></ErrorBoundary> },
          { path: '/settings/password', element: <ErrorBoundary><ChangePasswordPage /></ErrorBoundary> },
          { path: '/payments/history', element: <ErrorBoundary><PaymentHistoryPage /></ErrorBoundary> },
        ],
      },
    ],
  },

  // Admin routes
  {
    element: <AdminRoute />,
    children: [
      {
        element: <AdminLayout />,
        children: [
          { path: '/admin', element: <ErrorBoundary><AdminDashboardPage /></ErrorBoundary> },
          { path: '/admin/users', element: <ErrorBoundary><AdminUsersPage /></ErrorBoundary> },
          { path: '/admin/users/:id', element: <ErrorBoundary><AdminUserDetailPage /></ErrorBoundary> },
          { path: '/admin/promo', element: <ErrorBoundary><AdminPromoPage /></ErrorBoundary> },
          { path: '/admin/tickets', element: <ErrorBoundary><AdminTicketsPage /></ErrorBoundary> },
          { path: '/admin/tickets/:id', element: <ErrorBoundary><AdminTicketDetailPage /></ErrorBoundary> },
          { path: '/admin/payments', element: <ErrorBoundary><AdminPaymentsPage /></ErrorBoundary> },
          { path: '/admin/subscriptions', element: <ErrorBoundary><AdminSubscriptionsPage /></ErrorBoundary> },
          { path: '/admin/referrals', element: <ErrorBoundary><AdminReferralsPage /></ErrorBoundary> },
          { path: '/admin/yad', element: <ErrorBoundary><AdminYADPage /></ErrorBoundary> },
          { path: '/admin/notifications', element: <ErrorBoundary><AdminNotificationsPage /></ErrorBoundary> },
        ],
      },
    ],
  },

  // Catch-all → 404
  { path: '*', element: <NotFoundPage /> },
])
