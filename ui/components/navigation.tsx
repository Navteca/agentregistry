"use client"

import { useEffect, useState } from "react"
import Link from "next/link"
import { usePathname } from "next/navigation"
import { ChevronDown, LogOut } from "lucide-react"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import {
  getActiveUserProfile,
  logoutFromKeycloak,
  type KeycloakUserProfile,
  subscribeKeycloakSession,
} from "@/lib/keycloak-session"
import { useTheme } from "next-themes"
import { useSyncExternalStore } from "react"
import { Moon, Sun } from "lucide-react"
import Image from "next/image"

export function Navigation() {
  const pathname = usePathname()
  const [isLoggingOut, setIsLoggingOut] = useState(false)
  const [userProfile, setUserProfile] = useState<KeycloakUserProfile | null>(() => getActiveUserProfile())

  useEffect(() => {
    return subscribeKeycloakSession(() => {
      setUserProfile(getActiveUserProfile())
    })
  }, [])
  const { theme, setTheme } = useTheme()
  const mounted = useSyncExternalStore(() => () => {}, () => true, () => false)

  const isActive = (path: string) => {
    if (path === "/") {
      return pathname === "/"
    }
    return pathname.startsWith(path)
  }

  const getLinkClasses = (path: string) => {
    const baseClasses = "text-sm font-medium transition-colors"
    if (isActive(path)) {
      return `${baseClasses} text-foreground hover:text-foreground/80 border-b-2 border-foreground pb-1`
    }
    return `${baseClasses} text-muted-foreground hover:text-foreground`
  }

  const handleLogout = async () => {
    setIsLoggingOut(true)
    try {
      await logoutFromKeycloak()
    } catch (error) {
      console.error("logout failed", error)
      window.location.reload()
    } finally {
      setIsLoggingOut(false)
    }
  }

  const accountLabel = userProfile?.name ?? "Authenticated user"
  const accountSubLabel = userProfile?.email ?? userProfile?.username ?? "Keycloak session"
  const accountInitials = userProfile?.initials ?? "AR"

  return (
    <nav className="border-b bg-background sticky top-0 z-50">
      <div className="container mx-auto px-6">
        <div className="flex items-center gap-10 h-14">
          <Link href="/" className="flex items-center shrink-0 rounded-md px-2 py-1">
            <Image
              src={mounted && theme === "dark" ? "/logo-dark.svg" : "/logo-light.svg"}
              alt="AIRegistry"
              width={180}
              height={60}
              className="h-12 w-auto"
            />
          </Link>

          <div className="flex items-center gap-1">
            <Link
              href="/"
              className={`relative px-3 py-1.5 text-[15px] font-medium transition-colors ${
                isActive("/")
                  ? "text-foreground after:absolute after:bottom-[-13px] after:left-1 after:right-1 after:h-[2px] after:bg-primary after:rounded-full"
                  : "text-muted-foreground hover:text-foreground"
              }`}
            >
              Catalog
            </Link>
            <Link
              href="/deployed"
              className={`relative px-3 py-1.5 text-[15px] font-medium transition-colors ${
                isActive("/deployed")
                  ? "text-foreground after:absolute after:bottom-[-13px] after:left-1 after:right-1 after:h-[2px] after:bg-primary after:rounded-full"
                  : "text-muted-foreground hover:text-foreground"
              }`}
            >
              Deployed
            </Link>
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button
                  variant="outline"
                  size="sm"
                  className="gap-2"
                  aria-label="Open account menu"
                >
                  <span className="inline-flex h-6 w-6 items-center justify-center rounded-full bg-muted text-[11px] font-semibold uppercase text-muted-foreground">
                    {accountInitials}
                  </span>
                  <span className="max-w-[140px] truncate">{accountLabel}</span>
                  <ChevronDown className="h-3.5 w-3.5 text-muted-foreground" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" className="w-64">
                <DropdownMenuLabel className="space-y-0.5">
                  <p className="text-sm font-medium leading-none">{accountLabel}</p>
                  <p className="text-xs font-normal text-muted-foreground truncate">{accountSubLabel}</p>
                </DropdownMenuLabel>
                <DropdownMenuSeparator />
                <DropdownMenuItem onClick={() => void handleLogout()} disabled={isLoggingOut}>
                  <LogOut className="mr-2 h-4 w-4" />
                  {isLoggingOut ? "Logging out..." : "Logout"}
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </div>

          <div className="ml-auto">
            {mounted && (
              <Button
                variant="ghost"
                size="icon"
                className="h-8 w-8"
                onClick={() => setTheme(theme === "dark" ? "light" : "dark")}
                title={`Switch to ${theme === "dark" ? "light" : "dark"} mode`}
              >
                {theme === "dark" ? (
                  <Sun className="h-4 w-4" />
                ) : (
                  <Moon className="h-4 w-4" />
                )}
              </Button>
            )}
          </div>
        </div>
      </div>
    </nav>
  )
}
