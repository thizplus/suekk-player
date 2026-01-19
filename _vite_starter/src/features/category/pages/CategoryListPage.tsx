import { useState } from 'react'
import { Plus, Pencil, Trash2, Loader2, FolderOpen, GripVertical, ChevronRight, Video } from 'lucide-react'
import {
  DndContext,
  closestCenter,
  KeyboardSensor,
  PointerSensor,
  useSensor,
  useSensors,
  type DragEndEvent,
} from '@dnd-kit/core'
import {
  arrayMove,
  SortableContext,
  sortableKeyboardCoordinates,
  useSortable,
  verticalListSortingStrategy,
} from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Empty,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
  EmptyDescription,
  EmptyContent,
} from '@/components/ui/empty'
import { useCategoriesTree, useCreateCategory, useUpdateCategory, useDeleteCategory, useReorderCategories } from '../hooks'
import type { Category, CategoryOrderItem } from '../types'
import { toast } from 'sonner'

// Form Component
function CategoryForm({
  category,
  parentId,
  onSubmit,
  onCancel,
  isLoading,
}: {
  category?: Category
  parentId?: string
  onSubmit: (data: { name: string; slug: string; parentId?: string }) => void
  onCancel: () => void
  isLoading: boolean
}) {
  const [name, setName] = useState(category?.name || '')
  const [slug, setSlug] = useState(category?.slug || '')
  const [slugError, setSlugError] = useState('')

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (!name.trim() || !slug.trim()) {
      toast.error('กรุณากรอกข้อมูลให้ครบ')
      return
    }
    if (slugError) {
      toast.error('กรุณาแก้ไขข้อมูลที่ไม่ถูกต้อง')
      return
    }
    onSubmit({
      name: name.trim(),
      slug: slug.trim(),
      parentId,
    })
  }

  const handleSlugChange = (value: string) => {
    // Auto-format: lowercase, replace spaces with dash
    const formatted = value.toLowerCase().replace(/\s+/g, '-')
    setSlug(formatted)
    // Validate: only a-z, 0-9, dash
    if (formatted && !/^[a-z0-9-]+$/.test(formatted)) {
      setSlugError('ใช้ได้เฉพาะ a-z, 0-9, และ -')
    } else {
      setSlugError('')
    }
  }

  const handleNameChange = (value: string) => {
    setName(value)
    if (!category) {
      const autoSlug = value.toLowerCase().replace(/\s+/g, '-').replace(/[^a-z0-9-]/g, '')
      setSlug(autoSlug)
      setSlugError('')
    }
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-4">
      <div className="space-y-2">
        <Label htmlFor="name">ชื่อหมวดหมู่</Label>
        <Input
          id="name"
          value={name}
          onChange={(e) => handleNameChange(e.target.value)}
          placeholder="เช่น ภาพยนตร์, ซีรีส์"
          required
        />
      </div>
      <div className="space-y-2">
        <Label htmlFor="slug">Slug (URL)</Label>
        <Input
          id="slug"
          value={slug}
          onChange={(e) => handleSlugChange(e.target.value)}
          placeholder="เช่น movies, series"
          required
          className={slugError ? 'border-destructive' : ''}
        />
        {slugError && <p className="text-xs text-destructive">{slugError}</p>}
      </div>
      <div className="flex gap-2 justify-end">
        <Button type="button" variant="outline" onClick={onCancel} size="sm">
          ยกเลิก
        </Button>
        <Button type="submit" disabled={isLoading || !!slugError} size="sm">
          {isLoading && <Loader2 className="h-4 w-4 mr-2 animate-spin" />}
          {category ? 'บันทึก' : 'เพิ่ม'}
        </Button>
      </div>
    </form>
  )
}

// Sortable Category Item
function SortableCategoryItem({
  category,
  depth = 0,
  onEdit,
  onDelete,
  onAddChild,
}: {
  category: Category
  depth?: number
  onEdit: (category: Category) => void
  onDelete: (category: Category) => void
  onAddChild: (parentId: string) => void
}) {
  const {
    attributes,
    listeners,
    setNodeRef,
    transform,
    transition,
    isDragging,
  } = useSortable({ id: category.id })

  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.5 : 1,
  }

  return (
    <div ref={setNodeRef} style={style}>
      <div
        className="flex items-center gap-2 px-3 py-2.5 rounded-lg border border-dashed hover:bg-accent/50 transition-colors leading-none"
        style={{ marginLeft: depth * 24 }}
      >
        <button
          type="button"
          className="cursor-grab active:cursor-grabbing touch-none"
          {...attributes}
          {...listeners}
        >
          <GripVertical className="h-4 w-4 text-muted-foreground" />
        </button>

        {depth > 0 && <ChevronRight className="h-3 w-3 text-muted-foreground" />}

        <FolderOpen className="h-4 w-4 text-muted-foreground shrink-0" />

        <div className="flex-1 min-w-0">
          <p className="font-medium truncate">{category.name}</p>
          <p className="flex items-center gap-3 text-xs text-muted-foreground mt-0.5">
            <span className="font-mono">/{category.slug}</span>
            <span className="inline-flex items-center gap-1">
              <Video className="h-3 w-3" />
              {category.videoCount} วิดีโอ
            </span>
          </p>
        </div>

        <div className="flex items-center gap-1 shrink-0">
          <Button
            variant="ghost"
            size="icon"
            className="h-7 w-7"
            onClick={() => onAddChild(category.id)}
            title="เพิ่มหมวดหมู่ย่อย"
          >
            <Plus className="h-3.5 w-3.5" />
          </Button>
          <Button
            variant="ghost"
            size="icon"
            className="h-7 w-7"
            onClick={() => onEdit(category)}
          >
            <Pencil className="h-3.5 w-3.5" />
          </Button>
          <Button
            variant="ghost"
            size="icon"
            className="h-7 w-7 text-destructive hover:text-destructive"
            onClick={() => onDelete(category)}
          >
            <Trash2 className="h-3.5 w-3.5" />
          </Button>
        </div>
      </div>

      {/* Render children */}
      {category.children && category.children.length > 0 && (
        <div className="mt-1 space-y-1">
          {category.children.map((child) => (
            <SortableCategoryItem
              key={child.id}
              category={child}
              depth={depth + 1}
              onEdit={onEdit}
              onDelete={onDelete}
              onAddChild={onAddChild}
            />
          ))}
        </div>
      )}
    </div>
  )
}

// Flatten tree to array with order info
function flattenTree(categories: Category[], parentId: string | null = null): CategoryOrderItem[] {
  const result: CategoryOrderItem[] = []
  categories.forEach((cat, index) => {
    result.push({
      id: cat.id,
      parentId: parentId,
      sortOrder: index,
    })
    if (cat.children && cat.children.length > 0) {
      result.push(...flattenTree(cat.children, cat.id))
    }
  })
  return result
}

export function CategoryListPage() {
  const { data: categories, isLoading } = useCategoriesTree()
  const createCategory = useCreateCategory()
  const updateCategory = useUpdateCategory()
  const deleteCategory = useDeleteCategory()
  const reorderCategories = useReorderCategories()

  const [isCreateOpen, setIsCreateOpen] = useState(false)
  const [createParentId, setCreateParentId] = useState<string | undefined>()
  const [editingCategory, setEditingCategory] = useState<Category | null>(null)

  const sensors = useSensors(
    useSensor(PointerSensor),
    useSensor(KeyboardSensor, {
      coordinateGetter: sortableKeyboardCoordinates,
    })
  )

  const handleCreate = async (data: { name: string; slug: string; parentId?: string }) => {
    try {
      await createCategory.mutateAsync(data)
      toast.success('เพิ่มหมวดหมู่สำเร็จ')
      setIsCreateOpen(false)
      setCreateParentId(undefined)
    } catch {
      toast.error('เกิดข้อผิดพลาด')
    }
  }

  const handleUpdate = async (data: { name: string; slug: string }) => {
    if (!editingCategory) return
    try {
      await updateCategory.mutateAsync({
        id: editingCategory.id,
        data: { name: data.name, slug: data.slug },
      })
      toast.success('บันทึกสำเร็จ')
      setEditingCategory(null)
    } catch {
      toast.error('เกิดข้อผิดพลาด')
    }
  }

  const handleDelete = async (category: Category) => {
    if (!confirm(`คุณต้องการลบหมวดหมู่ "${category.name}" หรือไม่?${category.children?.length ? '\n(หมวดหมู่ย่อยจะถูกลบด้วย)' : ''}`)) return
    try {
      await deleteCategory.mutateAsync(category.id)
      toast.success('ลบสำเร็จ')
    } catch {
      toast.error('ไม่สามารถลบหมวดหมู่ที่มีวิดีโอได้')
    }
  }

  const handleAddChild = (parentId: string) => {
    setCreateParentId(parentId)
    setIsCreateOpen(true)
  }

  const handleDragEnd = async (event: DragEndEvent) => {
    const { active, over } = event

    if (over && active.id !== over.id && categories) {
      const oldIndex = categories.findIndex((c) => c.id === active.id)
      const newIndex = categories.findIndex((c) => c.id === over.id)

      if (oldIndex !== -1 && newIndex !== -1) {
        const newCategories = arrayMove(categories, oldIndex, newIndex)
        const orderItems = flattenTree(newCategories)

        try {
          await reorderCategories.mutateAsync({ categories: orderItems })
        } catch {
          toast.error('ไม่สามารถจัดเรียงใหม่ได้')
        }
      }
    }
  }

  const categoryIds = categories?.map((c) => c.id) ?? []

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold">หมวดหมู่</h1>
          <p className="text-sm text-muted-foreground">
            {categories ? `${categories.length} รายการ` : 'จัดการหมวดหมู่วิดีโอ'}
          </p>
        </div>
        <Button size="sm" onClick={() => { setCreateParentId(undefined); setIsCreateOpen(true) }}>
          <Plus className="h-4 w-4 mr-2" />
          เพิ่มหมวดหมู่
        </Button>
      </div>

      {/* Category List */}
      {isLoading ? (
        <div className="flex justify-center py-12">
          <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
        </div>
      ) : categories && categories.length === 0 ? (
        <Empty className="border">
          <EmptyHeader>
            <EmptyMedia variant="icon">
              <FolderOpen className="h-6 w-6" />
            </EmptyMedia>
            <EmptyTitle>ยังไม่มีหมวดหมู่</EmptyTitle>
            <EmptyDescription>
              เริ่มสร้างหมวดหมู่แรกของคุณ
            </EmptyDescription>
          </EmptyHeader>
          <EmptyContent>
            <Button size="sm" onClick={() => setIsCreateOpen(true)}>
              <Plus className="h-4 w-4 mr-2" />
              เพิ่มหมวดหมู่
            </Button>
          </EmptyContent>
        </Empty>
      ) : (
        <DndContext
          sensors={sensors}
          collisionDetection={closestCenter}
          onDragEnd={handleDragEnd}
        >
          <SortableContext items={categoryIds} strategy={verticalListSortingStrategy}>
            <div className="space-y-1">
              {categories?.map((category) => (
                <SortableCategoryItem
                  key={category.id}
                  category={category}
                  onEdit={setEditingCategory}
                  onDelete={handleDelete}
                  onAddChild={handleAddChild}
                />
              ))}
            </div>
          </SortableContext>
        </DndContext>
      )}

      {/* Create Dialog */}
      <Dialog open={isCreateOpen} onOpenChange={(open) => { setIsCreateOpen(open); if (!open) setCreateParentId(undefined) }}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>
              {createParentId ? 'เพิ่มหมวดหมู่ย่อย' : 'เพิ่มหมวดหมู่ใหม่'}
            </DialogTitle>
          </DialogHeader>
          <CategoryForm
            parentId={createParentId}
            onSubmit={handleCreate}
            onCancel={() => { setIsCreateOpen(false); setCreateParentId(undefined) }}
            isLoading={createCategory.isPending}
          />
        </DialogContent>
      </Dialog>

      {/* Edit Dialog */}
      <Dialog open={!!editingCategory} onOpenChange={() => setEditingCategory(null)}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>แก้ไขหมวดหมู่</DialogTitle>
          </DialogHeader>
          {editingCategory && (
            <CategoryForm
              category={editingCategory}
              onSubmit={handleUpdate}
              onCancel={() => setEditingCategory(null)}
              isLoading={updateCategory.isPending}
            />
          )}
        </DialogContent>
      </Dialog>
    </div>
  )
}
