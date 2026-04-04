import { createBrowserRouter } from 'react-router-dom'
import { PrivateRoute } from './PrivateRoute'
import { AdminRoute } from './AdminRoute'
import { AppLayout } from '@/components/layout/AppLayout'
import { AdminLayout } from '@/components/layout/AdminLayout'

// Auth pages
import { LoginPage } from '@/pages/auth/LoginPage'
import { RegisterPage } from '@/pages/auth/RegisterPage'

// User pages
import { DashboardPage } from '@/pages/user/DashboardPage'
import { SubscriptionsPage } from '@/pages/user/SubscriptionsPage'
import { RenewalPage } from '@/pages/user/RenewalPage'
import { ReferralsPage } from '@/pages/user/ReferralsPage'
import { BalancePage } from '@/pages/user/BalancePage'
import { ShopPage } from '@/pages/user/ShopPage'
import { TicketsPage } from '@/pages/user/TicketsPage'
import { TicketDetailPage } from '@/pages/user/TicketDetailPage'
import { PromoPage } from '@/pages/user/PromoPage'

// Admin pages
import { AdminDashboardPage } from '@/pages/admin/AdminDashboardPage'
import { AdminUsersPage } from '@/pages/admin/AdminUsersPage'
import { AdminUserDetailPage } from '@/pages/admin/AdminUserDetailPage'
import { AdminPromoPage } from '@/pages/admin/AdminPromoPage'
import { AdminTicketsPage } from '@/pages/admin/AdminTicketsPage'
import { AdminTicketDetailPage } from '@/pages/admin/AdminTicketDetailPage'

import { Navigate } from 'react-router-dom'

export const router = createBrowserRouter([
  // Public routes
  { path: '/login', element: <LoginPage /> },
  { path: '/register', element: <RegisterPage /> },
  { path: '/', element: <Navigate to="/dashboard" replace /> },

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
          { path: '/balance', element: <BalancePage /> },
          { path: '/shop', element: <ShopPage /> },
          { path: '/tickets', element: <TicketsPage /> },
          { path: '/tickets/:id', element: <TicketDetailPage /> },
          { path: '/promo', element: <PromoPage /> },
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
        ],
      },
    ],
  },

  // Catch-all
  { path: '*', element: <Navigate to="/dashboard" replace /> },
])
