    default rel
section .data
    fmt db "setb: %lld, seta: %lld, sete: %lld", 10, 0
    flt_1 dq 0x3FF0000000000000  ; 1.0
    flt_2 dq 0x4000000000000000  ; 2.0
section .text
    global main
    extern printf
main:
    push rbp
    mov rbp, rsp
    sub rsp, 32
    
    movsd xmm1, [flt_1]
    movsd xmm0, [flt_2]
    
    xor rbx, rbx
    xor rdx, rdx
    xor r8, r8
    
    comisd xmm1, xmm0
    setb bl
    seta dl
    sete r8b
    
    mov r9, rbx  ; r9 = setb
    lea rcx, [fmt]
    call printf
    
    xor eax, eax
    add rsp, 32
    pop rbp
    ret
