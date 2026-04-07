import { createBrowserRouter } from 'react-router-dom'
import { PrivateRoute } from './PrivateRoute'
import { AdminRoute } from './AdminRoute'
import { AppLayout } from '@/components/layout/AppLayout'
import { AdminLayout } from '@/components/layout/AdminLayout'
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
          { path: '/dashboard', element: <DashboardPage /> },
          { path: '/subscriptions', element: <SubscriptionsPage /> },
          { path: '/subscriptions/renew', element: <RenewalPage /> },
          { path: '/referrals', element: <ReferralsPage /> },
          { path: '/shop', element: <ShopPage /> },
          { path: '/tickets', element: <TicketsPage /> },
          { path: '/tickets/:id', element: <TicketDetailPage /> },
          { path: '/promo', element: <PromoPage /> },
          { path: '/balance', element: <BalancePage /> },
          { path: '/settings', element: <SettingsPage /> },
          { path: '/settings/password', element: <ChangePasswordPage /> },
          { path: '/payments/history', element: <PaymentHistoryPage /> },
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
          { path: '/admin', element: <AdminDashboardPage /> },
          { path: '/admin/users', element: <AdminUsersPage /> },
          { path: '/admin/users/:id', element: <AdminUserDetailPage /> },
          { path: '/admin/promo', element: <AdminPromoPage /> },
          { path: '/admin/tickets', element: <AdminTicketsPage /> },
          { path: '/admin/tickets/:id', element: <AdminTicketDetailPage /> },
          { path: '/admin/payments', element: <AdminPaymentsPage /> },
          { path: '/admin/subscriptions', element: <AdminSubscriptionsPage /> },
          { path: '/admin/referrals', element: <AdminReferralsPage /> },
          { path: '/admin/yad', element: <AdminYADPage /> },
        ],
      },
    ],
  },

  // Catch-all → 404
  { path: '*', element: <NotFoundPage /> },
])
