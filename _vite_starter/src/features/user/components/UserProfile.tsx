import { Card, CardContent, CardHeader } from '@/components/ui/card'
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar'
import { ROLE_LABELS } from '@/constants/enums'
import type { UserProfile as UserProfileType } from '../types'

interface UserProfileProps {
  user: UserProfileType
}

export function UserProfile({ user }: UserProfileProps) {
  const initials = user.name
    .split(' ')
    .map((n) => n[0])
    .join('')
    .toUpperCase()

  return (
    <Card>
      <CardHeader className="flex flex-row items-center gap-4">
        <Avatar className="h-16 w-16">
          <AvatarImage src={user.avatar} alt={user.name} />
          <AvatarFallback>{initials}</AvatarFallback>
        </Avatar>
        <div>
          <h2 className="text-xl font-semibold">{user.name}</h2>
          <p className="text-muted-foreground">{user.email}</p>
        </div>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="grid grid-cols-2 gap-4">
          <div>
            <p className="text-muted-foreground text-sm">บทบาท</p>
            <p className="font-medium">{ROLE_LABELS[user.role]}</p>
          </div>
          {user.phone && (
            <div>
              <p className="text-muted-foreground text-sm">เบอร์โทร</p>
              <p className="font-medium">{user.phone}</p>
            </div>
          )}
          {user.department && (
            <div>
              <p className="text-muted-foreground text-sm">แผนก</p>
              <p className="font-medium">{user.department}</p>
            </div>
          )}
        </div>
      </CardContent>
    </Card>
  )
}
