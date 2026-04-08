import { useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { SnakeLogo, Icon } from '@/components/ui/Icons'
import { useAuthStore } from '@/store/authStore'
import { SnakeBackground } from '@/components/SnakeBackground'

// ─── Landing Navbar ──────────────────────────────────────────────────────────

function LandingNav() {
  const [menuOpen, setMenuOpen] = useState(false)
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated())

  const links = [
    { href: '#features', label: 'Возможности' },
    { href: '#pricing', label: 'Цены' },
    { href: '#tech', label: 'Технические детали' },
    { href: '#faq', label: 'FAQ' },
  ]

  return (
    <header className="fixed inset-x-0 top-0 z-50 border-b border-surface-700/50 bg-surface-950/80 backdrop-blur-md">
      <div className="mx-auto flex h-16 max-w-6xl items-center justify-between px-4 sm:px-6">
        {/* Logo */}
        <Link
          to="/"
          className="flex items-center gap-2.5 font-bold text-slate-100 transition-colors hover:text-primary-400"
        >
          <SnakeLogo size={28} />
          <span className="text-lg tracking-tight">
            Mello<span className="text-primary-500">WPN</span>
          </span>
        </Link>

        {/* Desktop nav links */}
        <nav className="hidden items-center gap-7 md:flex">
          {links.map((l) => (
            <a
              key={l.href}
              href={l.href}
              className="text-sm text-slate-400 transition-colors hover:text-slate-100"
            >
              {l.label}
            </a>
          ))}
        </nav>

        {/* Desktop CTAs */}
        <div className="hidden items-center gap-3 md:flex">
          {isAuthenticated ? (
            <Link
              to="/dashboard"
              className="flex items-center gap-1.5 rounded-lg border border-primary-500/50 px-4 py-1.5 text-sm font-medium text-primary-400 transition-colors hover:bg-primary-500/10"
            >
              Личный кабинет →
            </Link>
          ) : (
            <>
              <Link
                to="/login"
                className="text-sm text-slate-400 transition-colors hover:text-slate-100"
              >
                Войти
              </Link>
              <Link
                to="/register"
                className="rounded-lg bg-primary-500 px-4 py-1.5 text-sm font-semibold text-white shadow-glow-sm transition-all hover:bg-primary-400 hover:shadow-glow-md"
              >
                Начать
              </Link>
            </>
          )}
        </div>

        {/* Mobile hamburger */}
        <button
          onClick={() => setMenuOpen(!menuOpen)}
          className="rounded-lg p-1.5 text-slate-400 transition-colors hover:text-slate-100 md:hidden"
          aria-label="Открыть меню"
        >
          <Icon name="menu" size={20} />
        </button>
      </div>

      {/* Mobile dropdown */}
      {menuOpen && (
        <div className="border-t border-surface-700 bg-surface-900 px-4 py-4 md:hidden">
          <nav className="flex flex-col gap-3">
            {links.map((l) => (
              <a
                key={l.href}
                href={l.href}
                onClick={() => setMenuOpen(false)}
                className="text-sm text-slate-300 transition-colors hover:text-slate-100"
              >
                {l.label}
              </a>
            ))}
          </nav>
          <div className="mt-4 flex flex-col gap-2 border-t border-surface-700 pt-4">
            {isAuthenticated ? (
              <Link
                to="/dashboard"
                className="rounded-lg border border-primary-500/50 px-4 py-2 text-center text-sm font-medium text-primary-400"
              >
                В личный кабинет →
              </Link>
            ) : (
              <>
                <Link
                  to="/login"
                  className="rounded-lg border border-surface-600 px-4 py-2 text-center text-sm text-slate-300"
                >
                  Войти
                </Link>
                <Link
                  to="/register"
                  className="rounded-lg bg-primary-500 px-4 py-2 text-center text-sm font-semibold text-white"
                >
                  Начать
                </Link>
              </>
            )}
          </div>
        </div>
      )}
    </header>
  )
}

// ─── Hero Section ─────────────────────────────────────────────────────────────

function HeroSection() {
  const navigate = useNavigate()

  const handleStart = () => navigate('/register')

  return (
    <section className="relative flex min-h-screen items-center overflow-hidden bg-surface-950 pt-16">
      {/* Snake animation background — sits below all other content */}
      <SnakeBackground />

      {/* Subtle background glow */}
      <div className="pointer-events-none absolute inset-0" aria-hidden="true">
        <div
          className="absolute left-1/2 top-0 h-[500px] w-[800px] -translate-x-1/2 rounded-full blur-3xl"
          style={{ background: 'radial-gradient(ellipse at center, rgba(34,197,94,0.05) 0%, transparent 70%)' }}
        />
      </div>

      <div className="relative mx-auto max-w-4xl px-4 py-28 text-center sm:px-6">
        {/* Logo mark */}
        <div className="mb-10 inline-flex items-center justify-center">
          <div className="flex h-20 w-20 items-center justify-center rounded-2xl border border-primary-900/60 bg-surface-900 shadow-glow-sm">
            <SnakeLogo size={48} />
          </div>
        </div>

        {/* Headline */}
        <h1 className="mx-auto max-w-4xl text-5xl font-extrabold leading-tight tracking-tight text-slate-100 sm:text-6xl md:text-7xl">
          Простой VPN.{' '}
          <span className="text-primary-500">Стабильное соединение.</span>
        </h1>

        {/* Subheadline */}
        <p className="mx-auto mt-6 max-w-2xl text-lg leading-relaxed text-slate-400 sm:text-xl">
          Без маркетинговых обещаний. Рабочий VPN с понятными тарифами и честной политикой.
        </p>

        {/* CTAs */}
        <div className="mt-9 flex flex-col items-center justify-center gap-3 sm:flex-row">
          <button
            onClick={handleStart}
            className="inline-flex items-center gap-2 rounded-xl bg-primary-500 px-8 py-3.5 text-base font-bold text-white shadow-glow-sm transition-all hover:bg-primary-400 hover:shadow-glow-md active:scale-95"
          >
            Начать
          </button>
          <a
            href="https://t.me/mellowpn_bot"
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center gap-2 rounded-xl border border-sky-500/40 bg-sky-500/10 px-8 py-3.5 text-base font-semibold text-sky-400 transition-all hover:bg-sky-500/20 hover:border-sky-500/60"
          >
            <svg width="18" height="18" viewBox="0 0 24 24" fill="currentColor"><path d="M12 0C5.373 0 0 5.373 0 12s5.373 12 12 12 12-5.373 12-12S18.627 0 12 0zm5.894 8.221-1.97 9.28c-.145.658-.537.818-1.084.508l-3-2.21-1.447 1.394c-.16.16-.295.295-.605.295l.213-3.053 5.56-5.023c.242-.213-.054-.333-.373-.12L8.32 14.347l-2.96-.924c-.643-.204-.657-.643.136-.953l11.57-4.461c.537-.194 1.006.131.828.212z"/></svg>
            Открыть бота
          </a>
          <a
            href="#pricing"
            className="inline-flex items-center gap-2 rounded-xl border border-surface-600 bg-surface-800 px-8 py-3.5 text-base font-semibold text-slate-300 transition-all hover:border-primary-500/40 hover:bg-surface-700 hover:text-slate-100"
          >
            Смотреть цены
          </a>
        </div>

        {/* Honest note */}
        <p className="mt-4 text-sm text-slate-600">
          Есть пробный период · Платные тарифы от 40 ₽/неделю
        </p>

        {/* Scroll hint */}
        <a
          href="#features"
          className="mt-12 inline-flex flex-col items-center gap-1 text-slate-600 transition-colors hover:text-slate-400"
        >
          <span className="text-xs tracking-widest">ЛИСТАТЬ</span>
          <Icon name="chevron-down" size={16} />
        </a>
      </div>

      {/* Live monitoring badge — bottom-left corner, purely decorative */}
      <div className="pointer-events-none absolute bottom-7 left-6 z-10 flex items-center gap-2.5 rounded-full border border-emerald-900/50 bg-surface-950/80 px-3.5 py-2 backdrop-blur-sm">
        <span className="relative flex h-2.5 w-2.5 flex-shrink-0">
          <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-emerald-400 opacity-60" />
          <span className="relative inline-flex h-2.5 w-2.5 rounded-full bg-emerald-500" />
        </span>
        <span className="text-xs font-medium tracking-wide text-slate-400">
          Блокируем угрозы. Защищаем твои данные.
        </span>
      </div>
    </section>
  )
}

// ─── Features Section ─────────────────────────────────────────────────────────

const features = [
  {
    icon: 'zap' as const,
    title: 'Стабильное соединение',
    desc: 'Протокол VLESS/Reality устойчив к обнаружению и обеспечивает надёжную работу без разрывов.',
  },
  {
    icon: 'settings' as const,
    title: 'Простая настройка',
    desc: 'Одна ссылка-подписка. Работает с Happ, V2RayN, Hiddify и любым VLESS-совместимым клиентом.',
  },
  {
    icon: 'eye-off' as const,
    title: 'Без рекламы',
    desc: 'Никаких трекеров, баннеров и попапов — ни в интерфейсе, ни в трафике.',
  },
  {
    icon: 'coins' as const,
    title: 'Предсказуемые цены',
    desc: 'Фиксированные тарифы. Нет автопродления без предупреждения, нет скрытых платежей.',
  },
  {
    icon: 'smartphone' as const,
    title: 'Все платформы',
    desc: 'iOS, Android, Windows, macOS, Linux — один конфиг работает на всех устройствах.',
  },
  {
    icon: 'message' as const,
    title: 'Telegram-бот',
    desc: 'Управляйте подпиской прямо в Telegram — бот @mellowpn_bot предоставляет те же возможности, что и сайт: покупка, продление, устройства, поддержка.',
  },
]

function FeaturesSection() {
  return (
    <section id="features" className="border-t border-surface-700/40 bg-surface-900 py-24">
      <div className="mx-auto max-w-6xl px-4 sm:px-6">
        <div className="text-center">
          <p className="text-xs font-semibold uppercase tracking-[0.2em] text-primary-500">
            Возможности
          </p>
          <h2 className="mt-3 text-3xl font-bold text-slate-100 md:text-4xl">
            Что вы получаете
          </h2>
          <p className="mx-auto mt-4 max-w-xl text-slate-400">
            Только реальные функции — без преувеличений.
          </p>
        </div>

        <div className="mt-14 grid gap-5 sm:grid-cols-2 lg:grid-cols-3">
          {features.map((f) => (
            <div
              key={f.title}
              className="group flex gap-4 rounded-2xl border border-surface-700 bg-surface-800 p-6 transition-all duration-200 hover:border-primary-500/40 hover:shadow-card-lg"
            >
              <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-primary-500/10 text-primary-500 transition-colors group-hover:bg-primary-500/20">
                <Icon name={f.icon} size={20} />
              </div>
              <div>
                <h3 className="font-semibold text-slate-100">{f.title}</h3>
                <p className="mt-1.5 text-sm leading-relaxed text-slate-400">{f.desc}</p>
              </div>
            </div>
          ))}
        </div>
      </div>
    </section>
  )
}

// ─── How It Works Section ─────────────────────────────────────────────────────

const steps = [
  {
    step: '01',
    icon: 'user' as const,
    title: 'Создайте аккаунт',
    desc: 'Регистрация за 30 секунд — только логин. Личные данные не нужны.',
  },
  {
    step: '02',
    icon: 'coins' as const,
    title: 'Выберите тариф',
    desc: 'Неделя, месяц или три месяца. Или начните с бесплатного периода.',
  },
  {
    step: '03',
    icon: 'shield' as const,
    title: 'Подключитесь',
    desc: 'Импортируйте конфиг в любой VPN-клиент. Защита включается за секунды.',
  },
]

function HowItWorksSection() {
  return (
    <section id="how-it-works" className="border-t border-surface-700/40 bg-surface-950 py-24">
      <div className="mx-auto max-w-6xl px-4 sm:px-6">
        <div className="text-center">
          <p className="text-xs font-semibold uppercase tracking-[0.2em] text-primary-500">
            Простой старт
          </p>
          <h2 className="mt-3 text-3xl font-bold text-slate-100 md:text-4xl">
            Подключитесь за 3 шага
          </h2>
          <p className="mx-auto mt-4 max-w-xl text-slate-400">
            Не нужно быть программистом. Если вы умеете установить приложение, вы справитесь с MelloWPN.
          </p>
        </div>

        <div className="mt-14 grid gap-6 md:grid-cols-3">
          {steps.map((s, i) => (
            <div key={s.step} className="relative">
              {/* Connector line */}
              {i < steps.length - 1 && (
                <div className="absolute left-full top-10 hidden h-px w-6 bg-gradient-to-r from-primary-500/40 to-transparent md:block" />
              )}
              <div className="rounded-2xl border border-surface-700 bg-surface-900 p-7">
                {/* Step badge */}
                <div className="mb-5 flex items-center gap-3">
                  <span className="flex h-9 w-9 items-center justify-center rounded-lg bg-primary-500/15 text-sm font-bold text-primary-500 ring-1 ring-primary-500/30">
                    {s.step}
                  </span>
                  <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-surface-700 text-slate-400">
                    <Icon name={s.icon} size={18} />
                  </div>
                </div>
                <h3 className="text-lg font-bold text-slate-100">{s.title}</h3>
                <p className="mt-2 text-sm leading-relaxed text-slate-400">{s.desc}</p>
              </div>
            </div>
          ))}
        </div>
      </div>
    </section>
  )
}

// ─── Pricing Section ──────────────────────────────────────────────────────────

interface PricingPlan {
  name: string
  price: string
  unit: string
  period: string
  badge?: string
  features: string[]
}

const plans: PricingPlan[] = [
  {
    name: 'Недельный',
    price: '₽40',
    unit: 'неделю',
    period: '7 дней доступа',
    features: [
      'Протокол VLESS/Reality',
      'Все доступные серверы',
      'IP-адреса не хранятся',
      'Все VLESS-совместимые клиенты',
      'Поддержка в Telegram',
    ],
  },
  {
    name: 'Месячный',
    price: '₽100',
    unit: 'месяц',
    period: '30 дней доступа',
    badge: 'Популярный',
    features: [
      'Всё из недельного',
      'Реферальная программа',
      'Бонусы ЯД',
      'Пробный период при регистрации',
    ],
  },
  {
    name: '3 Месяца',
    price: '₽270',
    unit: '3 месяца',
    period: '90 дней — экономия 30 ₽',
    badge: 'Выгоднее',
    features: [
      'Всё из месячного',
      'Двойные бонусы ЯД',
      'Экономия 30 ₽ против трёх месяцев',
    ],
  },
]

function PricingSection() {
  return (
    <section id="pricing" className="border-t border-surface-700/40 bg-surface-900 py-24">
      <div className="mx-auto max-w-6xl px-4 sm:px-6">
        <div className="text-center">
          <p className="text-xs font-semibold uppercase tracking-[0.2em] text-primary-500">
            Прозрачные цены
          </p>
          <h2 className="mt-3 text-3xl font-bold text-slate-100 md:text-4xl">
            Доступная защита
          </h2>
          <p className="mx-auto mt-4 max-w-xl text-slate-400">
            Без скрытых платежей и поднятия цен. Платите только за то, чем пользуетесь.
          </p>
        </div>

        <div className="mt-14 grid gap-6 md:grid-cols-3">
          {plans.map((plan) => {
            const isPopular = plan.badge === 'Популярный'
            return (
              <div
                key={plan.name}
                className={[
                  'relative flex flex-col rounded-2xl border p-7 transition-all',
                  isPopular
                    ? 'border-primary-500/50 bg-surface-800 shadow-glow-md'
                    : 'border-surface-700 bg-surface-800 hover:border-primary-500/30',
                ].join(' ')}
              >
                {/* Badge */}
                {plan.badge && (
                  <span
                    className={[
                      'absolute right-5 top-5 rounded-full px-2.5 py-0.5 text-[10px] font-bold uppercase tracking-widest',
                      isPopular
                        ? 'bg-primary-500/20 text-primary-400 ring-1 ring-primary-500/40'
                        : 'bg-surface-600 text-slate-400',
                    ].join(' ')}
                  >
                    {plan.badge}
                  </span>
                )}

                <div>
                  <p className="text-sm font-semibold uppercase tracking-wider text-slate-500">
                    {plan.name}
                  </p>
                  <div className="mt-3 flex items-end gap-1">
                    <span
                      className={[
                        'text-4xl font-extrabold',
                        isPopular ? 'text-primary-400' : 'text-slate-100',
                      ].join(' ')}
                    >
                      {plan.price}
                    </span>
                    <span className="mb-1 text-sm text-slate-500">/{plan.unit}</span>
                  </div>
                  <p className="mt-1 text-xs text-slate-600">{plan.period}</p>
                </div>

                <ul className="mt-6 flex-1 space-y-3">
                  {plan.features.map((feat) => (
                    <li key={feat} className="flex items-start gap-2.5 text-sm text-slate-300">
                      <Icon name="check" size={14} className="mt-0.5 shrink-0 text-primary-500" />
                      {feat}
                    </li>
                  ))}
                </ul>

                <Link
                  to="/register"
                  className={[
                    'mt-8 block rounded-xl py-3 text-center text-sm font-bold transition-all active:scale-95',
                    isPopular
                      ? 'bg-primary-500 text-white shadow-glow-sm hover:bg-primary-400 hover:shadow-glow-md'
                      : 'border border-surface-600 text-slate-300 hover:border-primary-500/40 hover:bg-surface-700 hover:text-slate-100',
                  ].join(' ')}
                >
                  Начать
                </Link>
              </div>
            )
          })}
        </div>

        <p className="mt-8 text-center text-sm text-slate-600">
          Автопродления нет. Оплата через СБП. Пробный период — при регистрации.
        </p>
      </div>
    </section>
  )
}

// ─── Technical Transparency Section ──────────────────────────────────────────

function TechTransparencySection() {
  return (
    <section id="tech" className="border-t border-surface-700/40 bg-surface-950 py-24">
      <div className="mx-auto max-w-4xl px-4 sm:px-6">
        <div className="text-center">
          <p className="text-xs font-semibold uppercase tracking-[0.2em] text-primary-500">
            Честно о технической стороне
          </p>
          <h2 className="mt-3 text-3xl font-bold text-slate-100 md:text-4xl">
            Как это работает
          </h2>
          <p className="mx-auto mt-4 max-w-xl text-slate-400">
            Без маркетинговых формулировок — только факты.
          </p>
        </div>

        <div className="mt-12 grid gap-4 sm:grid-cols-3">
          {/* Protocol */}
          <div className="rounded-2xl border border-surface-700 bg-surface-900 p-6">
            <div className="mb-3 flex h-10 w-10 items-center justify-center rounded-lg bg-primary-500/10 text-primary-500">
              <Icon name="server" size={20} />
            </div>
            <h3 className="font-semibold text-slate-100">Протокол</h3>
            <p className="mt-2 text-sm leading-relaxed text-slate-400">
              VLESS с транспортом Reality (xray-core). Не имеет специфичных заголовков VPN, трудно
              идентифицируем DPI-системами. Работает там, где блокируют классические VPN-протоколы.
            </p>
          </div>

          {/* Logs */}
          <div className="rounded-2xl border border-surface-700 bg-surface-900 p-6">
            <div className="mb-3 flex h-10 w-10 items-center justify-center rounded-lg bg-primary-500/10 text-primary-500">
              <Icon name="eye-off" size={20} />
            </div>
            <h3 className="font-semibold text-slate-100">Политика логов</h3>
            <p className="mt-2 text-sm leading-relaxed text-slate-400">
              IP-адреса пользователей и трафик не хранятся. Технические логи соединений (без
              содержимого) могут сохраняться до 24 часов для диагностики, затем удаляются.
              Данные аккаунта (логин) хранятся для доступа к сервису.
            </p>
          </div>

          {/* Limitations */}
          <div className="rounded-2xl border border-surface-700 bg-surface-900 p-6">
            <div className="mb-3 flex h-10 w-10 items-center justify-center rounded-lg bg-primary-500/10 text-primary-500">
              <Icon name="lock" size={20} />
            </div>
            <h3 className="font-semibold text-slate-100">Ограничения</h3>
            <p className="mt-2 text-sm leading-relaxed text-slate-400">
              Один активный конфиг на подписку. Сервис не гарантирует обход всех блокировок
              во всех странах и сетях. Скорость зависит от вашего провайдера и расстояния
              до сервера.
            </p>
          </div>
        </div>
      </div>
    </section>
  )
}

// ─── FAQ Section ──────────────────────────────────────────────────────────────

const faqs = [
  {
    q: 'Хранит ли MelloWPN логи?',
    a: 'IP-адреса и трафик не хранятся. Технические логи соединений (без содержимого) удаляются в течение 24 часов. Данные аккаунта (логин) хранятся для работы сервиса.',
  },
  {
    q: 'Как подключиться?',
    a: 'После оплаты вы получите ссылку-подписку. Импортируйте её в Happ, V2RayN, Hiddify или любой другой VLESS-совместимый клиент. Занимает около минуты.',
  },
  {
    q: 'Есть ли пробный период?',
    a: 'Да. После регистрации доступен пробный период без оплаты — можно проверить соединение перед покупкой.',
  },
  {
    q: 'Что такое ЯД?',
    a: 'ЯД — внутренняя валюта сервиса. Начисляется за рефералы (15% от платежа) и за промокоды сразу.',
  },
  {
    q: 'Можно ли отменить подписку?',
    a: 'Автопродления нет. Тариф действует ровно оплаченный период — после истечения просто не продлевайте.',
  },
  {
    q: 'Какие способы оплаты?',
    a: 'ЮМони и основные российские банковские карты через платёжный агрегатор.',
  },
]

function FaqSection() {
  const [openIndex, setOpenIndex] = useState<number | null>(null)

  return (
    <section id="faq" className="border-t border-surface-700/40 bg-surface-950 py-24">
      <div className="mx-auto max-w-3xl px-4 sm:px-6">
        <div className="text-center">
          <p className="text-xs font-semibold uppercase tracking-[0.2em] text-primary-500">
            Есть вопросы?
          </p>
          <h2 className="mt-3 text-3xl font-bold text-slate-100 md:text-4xl">
            Частые вопросы
          </h2>
        </div>

        <div className="mt-12 space-y-2">
          {faqs.map((faq, i) => {
            const isOpen = openIndex === i
            return (
              <div
                key={i}
                className={[
                  'overflow-hidden rounded-xl border transition-all duration-200',
                  isOpen
                    ? 'border-primary-500/40 bg-surface-800'
                    : 'border-surface-700 bg-surface-900 hover:border-surface-600',
                ].join(' ')}
              >
                <button
                  onClick={() => setOpenIndex(isOpen ? null : i)}
                  className="flex w-full items-center justify-between px-5 py-4 text-left"
                >
                  <span
                    className={[
                      'text-sm font-medium',
                      isOpen ? 'text-slate-100' : 'text-slate-300',
                    ].join(' ')}
                  >
                    {faq.q}
                  </span>
                  <span
                    className={[
                      'ml-4 shrink-0 text-slate-500 transition-transform duration-200',
                      isOpen ? 'rotate-180 text-primary-500' : '',
                    ].join(' ')}
                  >
                    <Icon name="chevron-down" size={16} />
                  </span>
                </button>
                {isOpen && (
                  <div className="px-5 pb-5">
                    <p className="text-sm leading-relaxed text-slate-400">{faq.a}</p>
                  </div>
                )}
              </div>
            )
          })}
        </div>
      </div>
    </section>
  )
}

// ─── Final CTA Section ────────────────────────────────────────────────────────

function FinalCtaSection() {
  return (
    <section className="border-t border-surface-700/40 bg-surface-900 py-24">
      <div className="mx-auto max-w-4xl px-4 text-center sm:px-6">
        {/* Glow */}
        <div
          className="pointer-events-none absolute left-1/2 -translate-x-1/2"
          style={{
            width: '600px',
            height: '300px',
            background: 'radial-gradient(ellipse at center, rgba(34,197,94,0.06) 0%, transparent 70%)',
          }}
          aria-hidden="true"
        />

        <div className="relative">
          <div className="mb-6 inline-flex h-16 w-16 items-center justify-center rounded-2xl border border-primary-500/30 bg-primary-500/10 shadow-glow-md">
            <SnakeLogo size={40} />
          </div>

          <h2 className="text-3xl font-extrabold text-slate-100 sm:text-4xl">
            Зарегистрируйтесь — это займёт минуту
          </h2>
          <p className="mx-auto mt-5 max-w-xl text-base text-slate-400">
            Пробный период без оплаты. Посмотрите сами, как работает сервис, прежде чем платить.
          </p>

          <div className="mt-10 flex flex-col items-center justify-center gap-4 sm:flex-row">
            <Link
              to="/register"
              className="inline-flex items-center gap-2 rounded-xl bg-primary-500 px-10 py-4 text-base font-bold text-white shadow-glow-sm transition-all hover:bg-primary-400 hover:shadow-glow-md active:scale-95"
            >
              Начать
            </Link>
            <Link
              to="/login"
              className="text-sm text-slate-400 transition-colors hover:text-slate-100"
            >
              Уже есть аккаунт? Войти →
            </Link>
          </div>

          <div className="mt-8 flex flex-wrap items-center justify-center gap-6 text-xs text-slate-600">
            {[
              'Пробный период без оплаты',
              'Автопродления нет',
              'IP-адреса не хранятся',
              'Отмена в любой момент',
            ].map((item) => (
              <span key={item} className="flex items-center gap-1.5">
                <Icon name="check" size={11} className="text-primary-500" />
                {item}
              </span>
            ))}
          </div>
        </div>
      </div>
    </section>
  )
}

// ─── Footer ───────────────────────────────────────────────────────────────────

function LandingFooter() {
  return (
    <footer className="border-t border-surface-700/40 bg-surface-950 py-12">
      <div className="mx-auto max-w-6xl px-4 sm:px-6">
        <div className="grid gap-8 sm:grid-cols-2 md:grid-cols-4">
          {/* Brand */}
          <div className="sm:col-span-2 md:col-span-1">
            <div className="flex items-center gap-2.5">
              <SnakeLogo size={24} />
              <span className="font-bold text-slate-100">
                Mello<span className="text-primary-500">WPN</span>
              </span>
            </div>
            <p className="mt-3 text-xs leading-relaxed text-slate-500">
              VPN-сервис на базе VLESS/Reality. Честная политика, отсутствие маркетинговых обещаний.
            </p>
          </div>

          {/* Product links */}
          <div>
            <p className="mb-3 text-xs font-semibold uppercase tracking-wider text-slate-500">
              Продукт
            </p>
            <ul className="space-y-2">
              {[
                { href: '#features', label: 'Возможности' },
                { href: '#pricing', label: 'Цены' },
                { href: '#tech', label: 'Технические детали' },
                { href: '#faq', label: 'FAQ' },
              ].map((l) => (
                <li key={l.href}>
                  <a
                    href={l.href}
                    className="text-sm text-slate-500 transition-colors hover:text-slate-300"
                  >
                    {l.label}
                  </a>
                </li>
              ))}
            </ul>
          </div>

          {/* Account links */}
          <div>
            <p className="mb-3 text-xs font-semibold uppercase tracking-wider text-slate-500">
              Аккаунт
            </p>
            <ul className="space-y-2">
              {[
                { to: '/register', label: 'Начать' },
                { to: '/login', label: 'Войти' },
                { to: '/dashboard', label: 'Личный кабинет' },
              ].map((l) => (
                <li key={l.to}>
                  <Link
                    to={l.to}
                    className="text-sm text-slate-500 transition-colors hover:text-slate-300"
                  >
                    {l.label}
                  </Link>
                </li>
              ))}
            </ul>
          </div>

          {/* Trust signals */}
          <div>
            <p className="mb-3 text-xs font-semibold uppercase tracking-wider text-slate-500">
              Безопасность
            </p>
            <ul className="space-y-2.5">
              {[
                { icon: 'server' as const, text: 'Протокол VLESS/Reality' },
                { icon: 'eye-off' as const, text: 'IP не хранится' },
                { icon: 'lock' as const, text: 'Трафик не логируется' },
              ].map((item) => (
                <li key={item.text} className="flex items-center gap-2 text-sm text-slate-500">
                  <Icon name={item.icon} size={13} className="shrink-0 text-primary-500/70" />
                  {item.text}
                </li>
              ))}
            </ul>
          </div>
        </div>

        {/* Bottom bar */}
        <div className="mt-10 flex flex-col items-center justify-between gap-3 border-t border-surface-700/40 pt-6 sm:flex-row">
          <p className="text-xs text-slate-600">
            © {new Date().getFullYear()} MelloWPN. Все права защищены.
          </p>
          <div className="flex items-center gap-4">
            <a href="/PrivacyPolicy" className="text-xs text-slate-600 hover:text-primary-500 transition-colors">
              Политика конфиденциальности
            </a>
            <a href="/UserAgreement" className="text-xs text-slate-600 hover:text-primary-500 transition-colors">
              Пользовательское соглашение
            </a>
          </div>
        </div>
      </div>
    </footer>
  )
}

// ─── Page Export ──────────────────────────────────────────────────────────────

export function LandingPage() {
  return (
    <div className="min-h-screen bg-surface-950 text-slate-200">
      <LandingNav />
      <main>
        <HeroSection />
        <FeaturesSection />
        <HowItWorksSection />
        <PricingSection />
        <TechTransparencySection />
        <FaqSection />
        <FinalCtaSection />
      </main>
      <LandingFooter />
    </div>
  )
}
